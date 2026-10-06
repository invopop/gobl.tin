package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/invopop/gobl.tin/aeat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"software.sslmate.com/src/go-pkcs12"
)

// writeCertificate writes a self-signed certificate as a PKCS#12 file and
// returns its path.
func writeCertificate(t *testing.T, password string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "gobl.tin test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	data, err := pkcs12.Modern.Encode(key, leaf, nil, password)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "cert.p12")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func TestAEATRegister(t *testing.T) {
	t.Run("no certificate is not configured", func(t *testing.T) {
		v, err := (&aeatRegister{}).verifier()
		require.NoError(t, err)
		assert.Nil(t, v)
	})

	t.Run("a certificate builds the AEAT verifier", func(t *testing.T) {
		t.Setenv(envAEATPassword, "secret")
		v, err := (&aeatRegister{cert: writeCertificate(t, "secret"), seal: true}).verifier()
		require.NoError(t, err)
		require.NotNil(t, v)
		assert.Equal(t, aeat.Source, v.Source())
	})

	t.Run("a wrong password is an error", func(t *testing.T) {
		t.Setenv(envAEATPassword, "wrong")
		_, err := (&aeatRegister{cert: writeCertificate(t, "secret")}).verifier()
		assert.ErrorContains(t, err, "decoding AEAT certificate")
	})

	t.Run("a missing file is a usage error", func(t *testing.T) {
		_, err := runVerify(t, nil, "--aeat-cert", filepath.Join(t.TempDir(), "missing.p12"), "../../test/data/party.json")
		assert.ErrorContains(t, err, "reading AEAT certificate")
		assert.Equal(t, exitUsage, exitCode(err))
	})
}
