package main

import (
	"crypto/tls"
	"fmt"
	"os"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/aeat"
	"github.com/spf13/pflag"
	"software.sslmate.com/src/go-pkcs12"
)

// envAEATPassword names the environment variable with the password of the
// AEAT certificate, so that the password never appears in the process list.
const envAEATPassword = "AEAT_CERT_PASSWORD"

// aeatRegister wires AEAT. It runs only when a certificate is given, because
// AEAT requires mutual TLS.
type aeatRegister struct {
	cert string
	seal bool
}

func (r *aeatRegister) flags(fs *pflag.FlagSet) {
	fs.StringVar(&r.cert, "aeat-cert", "", "PKCS#12 certificate for AEAT, which then verifies Spanish tax IDs; the password is read from "+envAEATPassword)
	fs.BoolVar(&r.seal, "aeat-seal", false, "The AEAT certificate is a seal certificate")
}

func (r *aeatRegister) verifier() (tin.Verifier, error) {
	if r.cert == "" {
		return nil, nil
	}
	cert, err := loadCertificate(r.cert, os.Getenv(envAEATPassword))
	if err != nil {
		return nil, err
	}
	var opts []aeat.Option
	if r.seal {
		opts = append(opts, aeat.WithSeal())
	}
	return aeat.New(cert, opts...), nil
}

// loadCertificate reads a PKCS#12 file into a TLS certificate with its
// chain.
func loadCertificate(path, password string) (tls.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("reading AEAT certificate: %w", err)
	}
	key, leaf, chain, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("decoding AEAT certificate: %w", err)
	}
	cert := tls.Certificate{Certificate: [][]byte{leaf.Raw}, PrivateKey: key, Leaf: leaf}
	for _, ca := range chain {
		cert.Certificate = append(cert.Certificate, ca.Raw)
	}
	return cert, nil
}
