package tls

import "crypto/tls"

type fromFile struct {
	certFiles          []string
	keyFiles           []string
	insecureSkipVerify bool
}

type FromFileOption func(*fromFile)

func FromFileWithCertificate(certFile, keyFile string) FromFileOption {
	return func(f *fromFile) {
		f.certFiles = append(f.certFiles, certFile)
		f.keyFiles = append(f.keyFiles, keyFile)
	}
}

func FromFileWithInsecureSkipVerify(insecureSkipVerify bool) FromFileOption {
	return func(f *fromFile) {
		f.insecureSkipVerify = insecureSkipVerify
	}
}

func FromFile(opts ...FromFileOption) (TlsContext, error) {

	o := &fromFile{
		certFiles: []string{},
		keyFiles:  []string{},
	}

	for _, opt := range opts {
		opt(o)
	}

	certs := make([]tls.Certificate, 0)

	for i := 0; i < len(o.certFiles); i++ {

		cert, err := tls.LoadX509KeyPair(o.certFiles[i], o.keyFiles[i])
		if err != nil {
			return nil, err
		}

		certs = append(certs, cert)
	}

	tlsCfg := &tls.Config{
		Certificates:       certs,
		ClientAuth:         tls.RequestClientCert,
		InsecureSkipVerify: o.insecureSkipVerify,
		NextProtos:         []string{"h2"},
	}

	return func() *tls.Config {
		return tlsCfg
	}, nil
}
