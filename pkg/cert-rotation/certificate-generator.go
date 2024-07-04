/*
Copyright 2024 Vesoft Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	cron "github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cbc "k8s.io/component-base/config"
	"k8s.io/klog/v2"
	ctrlruntime "sigs.k8s.io/controller-runtime"
)

const (
	ResourceName = "nebula-certificate-generator"
)

type CertGenerator struct {
	// LeaderElection defines the configuration of leader election client.
	LeaderElection cbc.LeaderElectionConfiguration

	// WebhookNames represents the names of the webhooks in the webhook server (i.e. controller-manager-nebula-operator-webhook, autoscaler-nebula-operator-webhook)
	WebhookNames *[]string

	// WebhookServerName represents the name of the webhook server associated with the certificate.
	WebhookServerName string

	// WebhookNamespace represents the namespace of the webhook server associated with the certificate.
	WebhookNamespace string

	// CertDir represents the directory to save the certificates in
	CertDir string

	// CertValidity represents the duration the certificate should be valid for
	CertValidity string

	// SecretName represents the name of the secret used to store the webhook certificates
	SecretName string

	// SecretNamespace represents the namespace of the secret used to store the webhook certificates
	SecretNamespace string

	// KubernetesDomain represents the custom kubernetes domain needed in the certificate
	KubernetesDomain string
}

func (c *CertGenerator) Run(ctx context.Context) error {
	klog.Info("Getting kubernetes configs")
	cfg, err := ctrlruntime.GetConfig()
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Errorf("Error building Kubernetes clientset: %v", err)
		return err
	}

	certValidityDuration, err := time.ParseDuration(c.CertValidity)
	if err != nil {
		klog.Errorf("Invalid cert validity %v, cert validity needs to be in the format 0h0m0s. Msg: %v", c.CertValidity, err)
		return err
	}

	// Check for minimum cert validity to avoid race condition (i.e. next certificate rotations starts before previous one if complete.)
	if certValidityDuration < 5*time.Minute {
		return fmt.Errorf("invalid cert validity %v, cert validity needs to be at least 2 minutes", c.CertValidity)
	}

	// Initialize certificate
	err = c.rotateCert(ctx, clientset, certValidityDuration)
	if err != nil {
		klog.Errorf("Error rotating certificate for webhook [%v/%v]: %v", c.WebhookNamespace, c.WebhookServerName, err)
		return err
	}

	// Start background job for rotation
	klog.Infof("Starting cert rotation cronjob for webhook [%v/%v] every %v", c.WebhookNamespace, c.WebhookServerName, (certValidityDuration - 30*time.Second).String())
	rotateJob := cron.New()
	rotateJob.AddFunc(fmt.Sprintf("@every %v", certValidityDuration-30*time.Second), func() {
		klog.Infof("Rotating certificate for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
		err := c.rotateCert(ctx, clientset, certValidityDuration+30*time.Second)
		if err != nil {
			klog.Errorf("Error rotating certificate for webhook [%v/%v]: %v", c.WebhookNamespace, c.WebhookServerName, err)
			os.Exit(1)
		}
		klog.Infof("Certifcate rotation complete for webhook [%v/%v]. Will rotate in %v", c.WebhookNamespace, c.WebhookServerName, certValidityDuration)
	})
	rotateJob.Start()
	klog.Infof("Cert rotation cronjob for webhook [%v/%v] started successfully", c.WebhookNamespace, c.WebhookServerName)

	return nil
}

func (c *CertGenerator) rotateCert(ctx context.Context, clientset *kubernetes.Clientset, certValidity time.Duration) error {
	klog.Info("Doing leader election")
	id, err := os.Hostname()
	if err != nil {
		klog.Errorf("Failed to get hostname: %v", err)
		return err
	}

	rl, err := resourcelock.New(
		c.LeaderElection.ResourceLock,
		c.LeaderElection.ResourceNamespace,
		ResourceName,
		clientset.CoreV1(),
		clientset.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: fmt.Sprintf("%v-%v", id, c.LeaderElection.ResourceName),
		},
	)
	if err != nil {
		klog.Errorf("Error creating resource lock: %v", err)
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: c.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: c.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   c.LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Info("Leader election successful. Starting certificate rotation")

				// Check certificate validity
				klog.Infof("Checking certificate validity for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
				currCertValidity, hasCert := c.getCertValidity("tls.crt")

				// Rotate if needed
				if !hasCert {
					klog.Infof("No certificate detected. Creating certificate for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
					err := c.doCertRotation(clientset, certValidity)
					if err != nil {
						klog.Errorf("Error rotating certificate for webhook [%v/%v]: %v", c.WebhookNamespace, c.WebhookServerName, err)
						os.Exit(1)
					}
					klog.Infof("Certificate created successfully for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
				} else if currCertValidity <= 1*time.Minute {
					klog.Infof("Certificate is within 1 min of expiration. Rotating certificate for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
					err := c.doCertRotation(clientset, currCertValidity+(certValidity))
					if err != nil {
						klog.Errorf("Error rotating certificate for webhook [%v/%v]: %v", c.WebhookNamespace, c.WebhookServerName, err)
						os.Exit(1)
					}
					klog.Infof("Certificate rotated successfully for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
				} else {
					klog.Infof("Certificate for webhook [%v/%v] is still valid for %v. Skipping cert rotation for now", c.WebhookNamespace, c.WebhookServerName, currCertValidity)
				}

				klog.Info("Finish certificate rotation. Relinquishing leadership")
				cancel()
			},
			OnStoppedLeading: func() {
				klog.Info("Lost leadership, stopping")
			},
		},
		ReleaseOnCancel: true,
	})

	return nil
}

func (c *CertGenerator) doCertRotation(clientset *kubernetes.Clientset, certValidity time.Duration) error {
	klog.Infof("Start generating certificates for webhook server [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
	caCert, caKey, err := c.generateCACert(certValidity)
	if err != nil {
		klog.Errorf("Error generating CA certificate for webhook server [%v/%v]: %v", c.WebhookNamespace, c.WebhookServerName, err)
		return err
	}

	serverCert, serverKey, err := c.generateServerCert(caCert, caKey, certValidity)
	if err != nil {
		klog.Errorf("Error generating server certificate for webhook server [%v/%v]: %v", c.WebhookNamespace, c.WebhookServerName, err)
		return err
	}

	err = c.updateSecret(clientset, serverCert, serverKey, caCert)
	if err != nil {
		klog.Errorf("Error updating secret for webhook server [%v/%v]: %v", c.WebhookNamespace, c.WebhookServerName, err)
		return err
	}
	klog.Infof("Certificates generated successfully for webhook server [%v/%v]", c.WebhookNamespace, c.WebhookServerName)

	klog.Infof("Updating ca bundle for webhook server [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
	err = c.updateWebhookConfiguration(clientset, caCert)
	if err != nil {
		klog.Errorf("Error updating ca bundle for webhook server [%v/%v]: %v", c.WebhookNamespace, c.WebhookServerName, err)
		return err
	}
	klog.Infof("Ca bundle updated successfully for webhook server [%v/%v]", c.WebhookNamespace, c.WebhookServerName)

	return nil
}

func (c *CertGenerator) generateCACert(certValidity time.Duration) ([]byte, *ecdsa.PrivateKey, error) {
	klog.V(4).Infof("Generating CA certificate and key for webhook server [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
	caKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:    []string{"US"},
			CommonName: c.WebhookServerName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(certValidity),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}

	caCert, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	caPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert})

	klog.V(4).Infof("CA certificate and key generated successfully for webhook server [%v/%v]", c.WebhookNamespace, c.WebhookServerName)

	return caPem, caKey, nil
}

func (c *CertGenerator) generateServerCert(caCert []byte, caKey *ecdsa.PrivateKey, certValidity time.Duration) ([]byte, []byte, error) {
	klog.V(4).Infof("Generating tls certificate and key for webhook server [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
	serverKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	dnsNames := make([]string, 2*len(*c.WebhookNames))
	for idx := -1; idx < len(*c.WebhookNames)-1; idx++ {
		dnsNames[idx+1] = fmt.Sprintf("%v.%v.svc", (*c.WebhookNames)[idx+1], c.WebhookNamespace)
		dnsNames[idx+2] = fmt.Sprintf("%v.%v.svc.%v", (*c.WebhookNames)[idx+1], c.WebhookNamespace, c.KubernetesDomain)
	}

	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Country:    []string{"US"},
			CommonName: c.WebhookServerName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(certValidity),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    dnsNames,
	}

	caBlock, _ := pem.Decode(caCert)
	ca, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	serverCert, err := x509.CreateCertificate(rand.Reader, &serverTemplate, ca, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCert})
	marshalledKey, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return nil, nil, fmt.Errorf("error marshalling webhook server certificate key: %v", err)
	}

	keyPem := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: marshalledKey})
	klog.V(4).Infof("TLS certificate and key generated successfully for webhook server [%v/%v]", c.WebhookNamespace, c.WebhookServerName)

	return certPem, keyPem, nil
}

func (c *CertGenerator) updateSecret(clientset *kubernetes.Clientset, certPEM, keyPEM, caPEM []byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.SecretName,
			Namespace: c.SecretNamespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": keyPEM,
			"ca.crt":  caPEM,
		},
	}

	// The mounted local directory in the pod will automatically refresh regardless of the sync-interval since we're updating the data field in the secret here
	if _, err := clientset.CoreV1().Secrets(c.SecretNamespace).Get(context.TODO(), c.SecretName, metav1.GetOptions{}); err == nil {
		klog.V(4).Infof("Updating secret [%v/%v] for webhook server [%v/%v]", c.SecretNamespace, c.SecretName, c.WebhookNamespace, c.WebhookServerName)
		_, err := clientset.CoreV1().Secrets(c.SecretNamespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update secret: %v", err)
		}
	} else {
		klog.V(4).Infof("No secret found. creating secret [%v/%v] for webhook server [%v/%v]", c.SecretNamespace, c.SecretName, c.WebhookNamespace, c.WebhookServerName)
		_, err := clientset.CoreV1().Secrets(c.SecretNamespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create secret: %w", err)
		}
	}

	// Wait for certificate in local volume to update
	err := c.waitForCertificateUpdate(filepath.Join(c.CertDir, "tls.crt"), certPEM)
	if err != nil {
		return fmt.Errorf("failed to wait for secret [%v/%v] to update: %v", c.SecretNamespace, c.SecretName, err)
	}

	klog.V(4).Infof("secret [%v/%v] updated successfully for webhook server [%v/%v]", c.SecretNamespace, c.SecretName, c.WebhookNamespace, c.WebhookServerName)

	return nil
}

func (c *CertGenerator) waitForCertificateUpdate(certPath string, expectedContent []byte) error {
	checkInterval := 2 * time.Second

	for {
		certBytes, err := os.ReadFile(certPath)
		if err != nil {
			return fmt.Errorf("unable to read certificate at path %v for webhook [%v/%v]: %v", certPath, c.WebhookNamespace, c.WebhookServerName, err)
		}

		block, _ := pem.Decode(certBytes)
		if block != nil && block.Type == "CERTIFICATE" && bytes.Contains(certBytes, expectedContent) {
			klog.V(4).Infof("Certificate updated successfully in the local volume for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
			return nil
		}

		klog.V(4).Infof("Waiting for certificate to be updated in the local volume for webhook [%v/%v], retrying in %v...", c.WebhookNamespace, c.WebhookServerName, checkInterval)
		time.Sleep(checkInterval)
	}
}

func (c *CertGenerator) updateWebhookConfiguration(client *kubernetes.Clientset, caCert []byte) error {
	webhook, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.Background(), c.WebhookServerName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get webhook configuration: %v", err)
	}

	for i := range webhook.Webhooks {
		webhook.Webhooks[i].ClientConfig.CABundle = caCert
	}

	_, err = client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(context.Background(), webhook, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update webhook configuration: %v", err)
	}

	return nil
}

func (c *CertGenerator) getCertValidity(certName string) (time.Duration, bool) {
	certPEM, err := os.ReadFile(filepath.Join(c.CertDir, certName))
	if err != nil {
		return -1, false
	}

	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return -1, false
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return -1, false
	}

	return time.Until(cert.NotAfter), true
}
