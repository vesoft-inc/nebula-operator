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
	"k8s.io/klog/v2"
	ctrlruntime "sigs.k8s.io/controller-runtime"
)

type CertGenerator struct {
	// WebhookNames represents the names of the webhooks in the webhook server (i.e. controller-manager-nebula-operator-webhook, autoscaler-nebula-operator-webhook)
	WebhookNames *[]string

	// WebhookServerName represents the name of the webhook server associated with the certificate.
	WebhookServerName string

	// WebhookNamespace represents the namespace of the webhook server associated with the certificate.
	WebhookNamespace string

	// CertDir represents the directory to save the certificates in
	CertDir string

	// CertValidity represents the number of days the certificate should be valid for
	CertValidity int64

	// SecretName represents the name of the secret used to store the webhook certificates
	SecretName string

	// SecretNamespace represents the namespace of the secret used to store the webhook certificates
	SecretNamespace string

	// KubernetesDomain represents the custom kubernetes domain needed in the certificate
	KubernetesDomain string
}

func (c *CertGenerator) Run() error {
	klog.Info("Getting kubernetes configs")
	cfg, err := ctrlruntime.GetConfig()
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Errorf("Error building Kubernetes clientset: %v", err.Error())
		return err
	}

	certValidityDuration := (time.Duration(c.CertValidity) * 24 * 60 * time.Minute)

	// Check certificate validity
	klog.Infof("Checking certificate validity for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
	currCertValidity, hasCert := c.getCertValidity("tls.cert")

	// Initialize certificate if needed
	if !hasCert {
		klog.Infof("No certificate detected. Creating certificate for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
		err := c.doCertRotation(clientset, certValidityDuration)
		if err != nil {
			klog.Errorf("Error rotating certificate for webhook [%v/%v]: %v", c.WebhookNamespace, c.WebhookServerName, err)
			os.Exit(1)
		}
		klog.Infof("Certificate created successfully for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
	} else if currCertValidity <= 2*time.Minute {
		klog.Infof("Certificate is within 2 min of expiration. Rotating certificate for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
		err := c.doCertRotation(clientset, currCertValidity+(certValidityDuration))
		if err != nil {
			klog.Errorf("Error rotating certificate for webhook [%v/%v]: %v", c.WebhookNamespace, c.WebhookServerName, err)
			os.Exit(1)
		}
		klog.Infof("Certificate rotated successfully for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
	}

	// Start background job for rotation
	rotateJob := cron.New()
	rotateJob.AddFunc(fmt.Sprintf("@every %v", certValidityDuration-1*time.Minute), func() {
		klog.Infof("Rotating certificate for webhook [%v/%v]", c.WebhookNamespace, c.WebhookServerName)
		err := c.doCertRotation(clientset, certValidityDuration+1*time.Minute)
		if err != nil {
			klog.Errorf("Error rotating certificate for webhook [%v/%v]: %v", c.WebhookNamespace, c.WebhookServerName, err)
			os.Exit(1)
		}
		klog.Infof("Certifcate rotation complete for webhook [%v/%v]. Will rotate in %v", c.WebhookNamespace, c.WebhookServerName, certValidityDuration)
	})
	rotateJob.Start()

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

	return caPem, caKey, nil
}

func (c *CertGenerator) generateServerCert(caCert []byte, caKey *ecdsa.PrivateKey, certValidity time.Duration) ([]byte, []byte, error) {
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
	_, err := clientset.CoreV1().Secrets(c.SecretNamespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret: %v", err)
	}

	// Wait for secret to update
	time.Sleep(15 * time.Second)

	return nil
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
