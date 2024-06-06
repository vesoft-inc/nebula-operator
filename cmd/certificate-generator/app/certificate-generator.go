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
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"os"
	"time"

	cron "github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
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
	}

	opts.CertValidity = opts.CertValidity * 24 * 60

	// rotate cert once before starting cronjob
	err = rotateCertificate(ctx, clientset, opts)
	if err != nil {
		klog.Errorf("Error rotating certificate for webhook [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookName, err)
		os.Exit(1)
	}

	klog.Infof("Starting cert rotation cron job for webhook [%v/%v]", opts.WebhookNamespace, opts.WebhookName)
	c := cron.New()
	// rotate cert 1 hour before expiration date
	c.AddFunc(fmt.Sprintf("@every %vm", opts.CertValidity-60), func() {
		err := rotateCertificate(ctx, clientset, opts)
		if err != nil {
			klog.Errorf("Error rotating certificate for webhook [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookName, err)
			os.Exit(1)
		}
	})
	c.Start()
	klog.Infof("Cert rotation crontab started for webhook [%v/%v]. Will rotate every %v minutes", opts.WebhookNamespace, opts.WebhookName, opts.CertValidity)

	// keep the program running
	select {}
}

func rotateCertificate(ctx context.Context, clientset *kubernetes.Clientset, opts *options.Options) error {
	if opts.LeaderElection.LeaderElect {
		klog.Info("Doing leader election")
		id, err := os.Hostname()
		if err != nil {
			klog.Errorf("Failed to get hostname: %v", err)
		}

		rl, err := resourcelock.New(resourcelock.LeasesResourceLock,
			opts.WebhookNamespace,
			opts.WebhookName,
			clientset.CoreV1(),
			clientset.CoordinationV1(),
			resourcelock.ResourceLockConfig{
				Identity: id,
			})
		if err != nil {
			klog.Errorf("Error creating resource lock: %v", err)
		}

		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:          rl,
			LeaseDuration: opts.LeaderElection.LeaseDuration.Duration,
			RenewDeadline: opts.LeaderElection.RenewDeadline.Duration,
			RetryPeriod:   opts.LeaderElection.RetryPeriod.Duration,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					klog.Info("Leader election successful. Starting certificate rotation")
					err = doCertRotation(clientset, opts)
					if err != nil {
						klog.Errorf("Failed to rotate certificates: %v", err)
					}
				},
				OnStoppedLeading: func() {
					klog.Info("Lost leadership, stopping")
				},
			},
		})
	} else {
		klog.Infof("Leader election skipped. Starting certificate rotation")
		err := doCertRotation(clientset, opts)
		if err != nil {
			klog.Errorf("Failed to rotate certificates: %v", err)
		}
	}

	return nil
}

func doCertRotation(clientset *kubernetes.Clientset, opts *options.Options) error {
	klog.Infof("Start generating certificates for webhook server [%v/%v]", opts.WebhookNamespace, opts.WebhookName)
	caCert, caKey, err := generateCACert(opts.WebhookName, opts.CertValidity)
	if err != nil {
		klog.Errorf("Error generating CA certificate for webhook server [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookName, err)
		return err
	}

	serverCert, serverKey, err := generateServerCert(caCert, caKey, opts.WebhookName, opts.WebhookNamespace, opts.CertValidity)
	if err != nil {
		klog.Errorf("Error generating server certificate for webhook server [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookName, err)
		return err
	}

	err = saveToFile(fmt.Sprintf("%v/ca.crt", opts.CertDir), "CERTIFICATE", caCert)
	if err != nil {
		klog.Errorf("Error saving CA certificate for webhook server [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookName, err)
		return err
	}

	err = saveToFile(fmt.Sprintf("%v/ca.key", opts.CertDir), "EC PRIVATE KEY", caKey)
	if err != nil {
		klog.Errorf("Error saving CA key for webhook server [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookName, err)
		return err
	}

	err = saveToFile(fmt.Sprintf("%v/tls.crt", opts.CertDir), "CERTIFICATE", serverCert)
	if err != nil {
		klog.Errorf("Error saving server certificate for webhook server [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookName, err)
		return err
	}

	err = saveToFile(fmt.Sprintf("%v/tls.key", opts.CertDir), "EC PRIVATE KEY", serverKey)
	if err != nil {
		klog.Errorf("Error saving server key for webhook server [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookName, err)
		return err
	}

	klog.Infof("Certificates generated successfully for webhook server [%v/%v]", opts.WebhookNamespace, opts.WebhookName)

	klog.Infof("Updating ca bundle for webhook server [%v/%v]", opts.WebhookNamespace, opts.WebhookName)
	err = updateWebhookConfiguration(clientset, opts.WebhookName, fmt.Sprintf("%v/ca.crt", opts.CertDir))
	if err != nil {
		klog.Errorf("Error updating ca bundle for webhook server [%v/%v]: %v", opts.WebhookNamespace, opts.WebhookName, err)
		return err
	}

	return nil
}

func generateCACert(webhookName string, validity int64) ([]byte, *ecdsa.PrivateKey, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:    []string{"US"},
			CommonName: webhookName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Duration(validity) * time.Minute),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCert, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	return caCert, caKey, nil
}

func generateServerCert(caCert []byte, caKey *ecdsa.PrivateKey, webhookName, webhookNamespace string, validity int64) ([]byte, *ecdsa.PrivateKey, error) {
	serverKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Country:    []string{"US"},
			CommonName: webhookName,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Duration(validity) * time.Minute),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{fmt.Sprintf("%v.%v.svc", webhookName, webhookNamespace), fmt.Sprintf("%v.%v.svc.cluster.local", webhookName, webhookNamespace)},
	}

	ca, err := x509.ParseCertificate(caCert)
	if err != nil {
		return nil, nil, err
	}

	serverCert, err := x509.CreateCertificate(rand.Reader, &serverTemplate, ca, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	return serverCert, serverKey, nil
}

func saveToFile(filename, blockType string, data interface{}) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	var pemData []byte
	switch data := data.(type) {
	case []byte:
		pemData = pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: data})
	case *ecdsa.PrivateKey:
		der, err := x509.MarshalECPrivateKey(data)
		if err != nil {
			return err
		}
		pemData = pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	default:
		return fmt.Errorf("unsupported data type")
	}

	_, err = file.Write(pemData)
	return err
}

// Update the webhook configuration with the new CA bundle
func updateWebhookConfiguration(client *kubernetes.Clientset, webhookName, caCertPath string) error {
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %v", err)
	}

	caBundle := base64.StdEncoding.EncodeToString(caCert)

	webhook, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.Background(), webhookName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get webhook configuration: %v", err)
	}

	for i := range webhook.Webhooks {
		webhook.Webhooks[i].ClientConfig.CABundle = []byte(caBundle)
	}

	_, err = client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(context.Background(), webhook, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update webhook configuration: %v", err)
	}

	return nil
}
