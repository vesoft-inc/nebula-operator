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
	"flag"
	"fmt"
	"math/big"
	"os"
	"time"

	cron "github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	ctrlruntime "sigs.k8s.io/controller-runtime"

	"github.com/vesoft-inc/nebula-operator/cmd/certificate-generator/app/options"
)

func NewCertGenCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()
	cmd := &cobra.Command{
		Use: "nebula-cert-gen",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(ctx, opts)
		},
	}

	nfs := cliflag.NamedFlagSets{}
	fs := nfs.FlagSet("generic")
	fs.AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(fs)
	logsFlagSet := nfs.FlagSet("logs")

	cmd.Flags().AddFlagSet(fs)
	cmd.Flags().AddFlagSet(logsFlagSet)

	return cmd
}

func Run(ctx context.Context, opts *options.Options) error {
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

	if opts.InitOnly {
		klog.Infof("Init only detected. Doing cert initialization for webhook [%v/%v]", opts.WebhookNamespace, opts.WebhookServerName)
		err := doCertRotation(clientset, opts)
		if err != nil {
			klog.Errorf("Error rotating certificate for webhook [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookServerName, err)
			return err
		}
	} else {
		if opts.LeaderElection.LeaderElect {
			klog.Info("Doing leader election")
			id, err := os.Hostname()
			if err != nil {
				klog.Errorf("Failed to get hostname: %v", err)
				return err
			}

			rl, err := resourcelock.New(opts.LeaderElection.ResourceLock,
				opts.LeaderElection.ResourceNamespace,
				opts.LeaderElection.ResourceName,
				clientset.CoreV1(),
				clientset.CoordinationV1(),
				resourcelock.ResourceLockConfig{
					Identity: id,
				})
			if err != nil {
				klog.Errorf("Error creating resource lock: %v", err)
				return err
			}

			leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
				Lock:          rl,
				LeaseDuration: opts.LeaderElection.LeaseDuration.Duration,
				RenewDeadline: opts.LeaderElection.RenewDeadline.Duration,
				RetryPeriod:   opts.LeaderElection.RetryPeriod.Duration,
				Callbacks: leaderelection.LeaderCallbacks{
					OnStartedLeading: func(ctx context.Context) {
						klog.Info("Leader election successful. Starting certificate rotation")
						err = rotateCertificate(clientset, opts)
						if err != nil {
							klog.Errorf("Failed to start cronjob to rotate certificates: %v", err)
							os.Exit(1)
						}
					},
					OnStoppedLeading: func() {
						klog.Info("Lost leadership, stopping")
					},
				},
			})
		} else {
			klog.Infof("Leader election skipped. Starting certificate rotation")
			err = rotateCertificate(clientset, opts)
			if err != nil {
				klog.Errorf("Failed to start cronjob to rotate certificates: %v", err)
				return err
			}
		}
	}

	return nil
}

func rotateCertificate(clientset *kubernetes.Clientset, opts *options.Options) error {
	opts.CertValidity = opts.CertValidity * 24 * 60

	klog.Infof("Starting cert rotation cron job for webhook [%v/%v]", opts.WebhookNamespace, opts.WebhookServerName)
	c := cron.New()
	// rotate cert 1 hour before expiration date
	_, err := c.AddFunc(fmt.Sprintf("@every %vm", opts.CertValidity-60), func() {
		err := doCertRotation(clientset, opts)
		if err != nil {
			klog.Errorf("Error rotating certificate for webhook [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookServerName, err)
			os.Exit(1)
		}
	})
	if err != nil {
		return err
	}
	klog.Infof("Cert rotation crontab started for webhook [%v/%v]. Will rotate every %v minutes", opts.WebhookNamespace, opts.WebhookServerName, opts.CertValidity)
	c.Run()

	return nil
}

func doCertRotation(clientset *kubernetes.Clientset, opts *options.Options) error {
	klog.Infof("Start generating certificates for webhook server [%v/%v]", opts.WebhookNamespace, opts.WebhookServerName)
	caCert, caKey, err := generateCACert(opts)
	if err != nil {
		klog.Errorf("Error generating CA certificate for webhook server [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookServerName, err)
		return err
	}

	serverCert, serverKey, err := generateServerCert(caCert, caKey, opts)
	if err != nil {
		klog.Errorf("Error generating server certificate for webhook server [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookServerName, err)
		return err
	}

	err = updateSecret(clientset, serverCert, serverKey, caCert, opts)
	if err != nil {
		klog.Errorf("Error updating secret for webhook server [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookServerName, err)
		return err
	}

	klog.Infof("Certificates generated successfully for webhook server [%v/%v]", opts.WebhookNamespace, opts.WebhookServerName)

	klog.Infof("Updating ca bundle for webhook server [%v/%v]", opts.WebhookNamespace, opts.WebhookServerName)
	err = updateWebhookConfiguration(clientset, opts, caCert)
	if err != nil {
		klog.Errorf("Error updating ca bundle for webhook server [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookServerName, err)
		return err
	}
	klog.Infof("Ca bundle updated successfully for webhook server [%v/%v]", opts.WebhookNamespace, opts.WebhookServerName)

	return nil
}

func generateCACert(opts *options.Options) ([]byte, *ecdsa.PrivateKey, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:    []string{"US"},
			CommonName: opts.WebhookServerName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Duration(opts.CertValidity) * time.Minute),
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

func generateServerCert(caCert []byte, caKey *ecdsa.PrivateKey, opts *options.Options) ([]byte, []byte, error) {
	serverKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	dnsNames := make([]string, 2*len(*opts.WebhookNames))
	for idx := -1; idx < len(*opts.WebhookNames)-1; idx++ {
		dnsNames[idx+1] = fmt.Sprintf("%v.%v.svc", (*opts.WebhookNames)[idx+1], opts.WebhookNamespace)
		dnsNames[idx+2] = fmt.Sprintf("%v.%v.svc.%v", (*opts.WebhookNames)[idx+1], opts.WebhookNamespace, opts.KubernetesDomain)
	}

	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Country:    []string{"US"},
			CommonName: opts.WebhookServerName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Duration(opts.CertValidity) * time.Minute),
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

func updateSecret(clientset *kubernetes.Clientset, certPEM, keyPEM, caPEM []byte, opts *options.Options) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.SecretName,
			Namespace: opts.SecretNamespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": keyPEM,
			"ca.crt":  caPEM,
		},
	}

	_, err := clientset.CoreV1().Secrets(opts.SecretNamespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret: %v", err)
	}

	return nil
}

// Update the webhook configuration with the new CA bundle
func updateWebhookConfiguration(client *kubernetes.Clientset, opts *options.Options, caCert []byte) error {
	webhook, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.Background(), opts.WebhookServerName, metav1.GetOptions{})
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
