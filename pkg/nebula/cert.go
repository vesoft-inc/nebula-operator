package nebula

import (
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

func getCerts(namespace string, cert *v1alpha1.SSLCertsSpec) ([]byte, []byte, []byte, error) {
	if os.Getenv("CA_CERT_PATH") != "" &&
		os.Getenv("CLIENT_CERT_PATH") != "" &&
		os.Getenv("CLIENT_KEY_PATH") != "" {
		caCert, err := os.ReadFile(os.Getenv("CA_CERT_PATH"))
		if err != nil {
			return nil, nil, nil, err
		}
		clientCert, err := os.ReadFile(os.Getenv("CLIENT_CERT_PATH"))
		if err != nil {
			return nil, nil, nil, err
		}
		clientKey, err := os.ReadFile(os.Getenv("CLIENT_KEY_PATH"))
		if err != nil {
			return nil, nil, nil, err
		}
		return caCert, clientCert, clientKey, nil
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	client, err := kube.NewClientSet(cfg)
	if err != nil {
		return nil, nil, nil, err
	}

	caSecret, err := client.Secret().GetSecret(namespace, cert.CASecret)
	if err != nil {
		return nil, nil, nil, err
	}
	caCert := caSecret.Data[cert.CACert]

	clientSecret, err := client.Secret().GetSecret(namespace, cert.ClientSecret)
	if err != nil {
		return nil, nil, nil, err
	}
	clientCert := clientSecret.Data[cert.ClientCert]
	clientKey := clientSecret.Data[cert.ClientKey]
	return caCert, clientCert, clientKey, nil
}
