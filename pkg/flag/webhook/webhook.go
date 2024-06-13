/*
Copyright 2023 Vesoft Inc.

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

package webhook

import (
	"net"

	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	defaultBindAddress   = "0.0.0.0"
	defaultPort          = 9443
	defaultCertDir       = "/tmp/k8s-webhook-server/serving-certs"
	defaultTLSMinVersion = "1.3"
)

// Options contains everything necessary to create and run webhook server.
type Options struct {
	// BindAddress is the IP address on which to listen for the --secure-port port.
	// Default is "0.0.0.0".
	BindAddress string

	// SecurePort is the port that the webhook server serves at.
	// Default is 8443.
	SecurePort int

	// CertDir is the directory that contains the server key and certificate.
	// if not set, webhook server would look up the server key and certificate in /tmp/k8s-webhook-server/serving-certs.
	CertDir string

	// CertName is the server certificate name. Defaults to tls.crt.
	CertName string

	// CertValidity represents the number of days the certificate should be valid for
	CertValidity int64

	// KeyName is the server key name. Defaults to tls.key.
	KeyName string

	// KubernetesDomain represents the custom kubernetes domain needed in the certificate
	KubernetesDomain string

	// SecretName represents the name of the secret used to store the webhook certificates
	SecretName string

	// SecretNamespace represents the namespace of the secret used to store the webhook certificates
	SecretNamespace string

	// TLSMinVersion is the minimum version of TLS supported. Possible values: 1.0, 1.1, 1.2, 1.3.
	// Some environments have automated security scans that trigger on TLS versions or insecure cipher suites, and
	// setting TLS to 1.3 would solve both problems.
	// Defaults to 1.3.
	TLSMinVersion string

	// WebhookNames represents the names of the webhooks in the webhook server (i.e. controller-manager-nebula-operator-webhook, autoscaler-nebula-operator-webhook)
	WebhookNames *[]string

	// WebhookServerName represents the name of the webhook server associated with the certificate.
	WebhookServerName string

	// WebhookNamespace represents the namespace of the webhook server associated with the certificate.
	WebhookNamespace string
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.BindAddress, "webhook-bind-address", defaultBindAddress,
		"The IP address on which to listen for the --secure-port port.")
	flags.IntVar(&o.SecurePort, "webhook-secure-port", defaultPort,
		"The secure port on which to serve HTTPS.")
	flags.StringVar(&o.CertDir, "webhook-cert-dir", defaultCertDir,
		"The directory that contains the server key and certificate.")
	flags.StringVar(&o.CertName, "webhook-tls-cert-file-name", "tls.crt", "The name of server certificate.")
	flags.Int64Var(&o.CertValidity, "certificate-validity", 365, "Specifies the number of days the certificate should be valid for")
	flags.StringVar(&o.KeyName, "webhook-tls-private-key-file-name", "tls.key", "The name of server key.")
	flags.StringVar(&o.KubernetesDomain, "kube-domain", "cluster.local", "Specifies the namespace of the webhook to associate with the certificate")
	flags.StringVar(&o.SecretName, "secret-name", "nebula-operator-webhook-secret", "Specifies the name of the webhook to associate with the certificate")
	flags.StringVar(&o.SecretNamespace, "secret-namespace", "default", "Specifies the namespace of the webhook to associate with the certificate")
	flags.StringVar(&o.TLSMinVersion, "webhook-tls-min-version", defaultTLSMinVersion,
		"Minimum TLS version supported. Possible values: 1.0, 1.1, 1.2, 1.3.")
	o.WebhookNames = flags.StringSlice("webhook-names", []string{}, "A comma-seperated list of the names of the webhooks supported by the webhook server (i.e. controller-manager-nebula-operator-webhook, autoscaler-nebula-operator-webhook)")
	flags.StringVar(&o.WebhookServerName, "webhook-server-name", "nebulaWebhook", "Specifies the name of the webhook to associate with the certificate")
	flags.StringVar(&o.WebhookNamespace, "webhook-namespace", "default", "Specifies the namespace of the webhook to associate with the certificate")
}

func (o *Options) Validate() field.ErrorList {
	errs := field.ErrorList{}

	newPath := field.NewPath("Options")
	if net.ParseIP(o.BindAddress) == nil {
		errs = append(errs, field.Invalid(newPath.Child("BindAddress"), o.BindAddress, "not a valid textual representation of an IP address"))
	}

	if o.SecurePort < 0 || o.SecurePort > 65535 {
		errs = append(errs, field.Invalid(newPath.Child("SecurePort"), o.SecurePort, "must be a valid port between 0 and 65535 inclusive"))
	}

	return errs
}
