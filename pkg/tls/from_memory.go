package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"log"
	"math/big"
	"time"
)

type fromMemory struct {
	organization     string
	organizationUnit string
}

type FromMemoryOption func(*fromMemory)

func FromMemoryWithOrganization(organization string) FromMemoryOption {
	return func(o *fromMemory) {
		o.organization = organization
	}
}

func FromMemoryWithOrganizationUnit(organizationUnit string) FromMemoryOption {
	return func(o *fromMemory) {
		o.organizationUnit = organizationUnit
	}
}

func FromMemory(opts ...FromMemoryOption) (TlsContext, error) {

	o := &fromMemory{
		organization:     "Acme Inc.",
		organizationUnit: "Acme Inc. TLS",
	}

	for _, opt := range opts {
		opt(o)
	}

	caCert := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization:       []string{o.organization},
			Country:            []string{},
			Province:           []string{},
			Locality:           []string{},
			OrganizationalUnit: []string{o.organizationUnit},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(10 * 360 * 24 * time.Hour),
		IsCA:      true,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, caCert, caCert, priv.PublicKey, priv)
	if err != nil {
		log.Panic("Error self signing certificate", "error", err)
	}

	selfSignedCert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		log.Panic("Error parsing self signed certificate", "error", err)
	}

	cert := tls.Certificate{
		Leaf:        selfSignedCert,
		Certificate: [][]byte{selfSignedCert.Raw},
		PrivateKey:  priv,
	}

	tlsCfg := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		ClientAuth:         tls.RequestClientCert,
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2"},
	}

	return func() *tls.Config {
		return tlsCfg
	}, nil

}
