package aeat

import (
	"crypto/tls"

	"software.sslmate.com/src/go-pkcs12"
)

// LoadCertificate decodes a PKCS#12 file, as the AEAT electronic office
// issues it, into a TLS certificate with its chain for New.
func LoadCertificate(data []byte, password string) (tls.Certificate, error) {
	key, leaf, chain, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		return tls.Certificate{}, err
	}
	cert := tls.Certificate{Certificate: [][]byte{leaf.Raw}, PrivateKey: key, Leaf: leaf}
	for _, ca := range chain {
		cert.Certificate = append(cert.Certificate, ca.Raw)
	}
	return cert, nil
}
