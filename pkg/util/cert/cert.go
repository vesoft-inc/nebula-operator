package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

func LoadTLSConfig(caCert, cert, key []byte) (*tls.Config, error) {
	c, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	rootCAPool := x509.NewCertPool()
	ok := rootCAPool.AppendCertsFromPEM(caCert)
	if !ok {
		return nil, fmt.Errorf("failed to load cert files")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{c},
		RootCAs:      rootCAPool,
	}, nil
}

func ValidCACert(key, cert, caCert []byte, dnsName string, time time.Time) bool {
	if len(key) == 0 || len(cert) == 0 || len(caCert) == 0 {
		return false
	}
	_, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return false
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return false
	}
	block, _ := pem.Decode(cert)
	if block == nil {
		return false
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	ops := x509.VerifyOptions{
		DNSName:     dnsName,
		Roots:       pool,
		CurrentTime: time,
	}
	_, err = c.Verify(ops)
	return err == nil
}
