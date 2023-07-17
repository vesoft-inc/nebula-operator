package nebula

import (
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/pkg/kube"
)

func getCerts(namespace string, cert *v1alpha1.SSLCertsSpec) ([]byte, []byte, []byte, error) {
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
