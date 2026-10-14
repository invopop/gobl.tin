package aeat

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"software.sslmate.com/src/go-pkcs12"
)

func TestLoadCertificate(t *testing.T) {
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
	data, err := pkcs12.Modern.Encode(key, leaf, nil, "secret")
	require.NoError(t, err)

	t.Run("decodes the key and the leaf", func(t *testing.T) {
		cert, err := LoadCertificate(data, "secret")
		require.NoError(t, err)
		assert.Equal(t, [][]byte{der}, cert.Certificate)
		assert.Equal(t, key, cert.PrivateKey)
		assert.Equal(t, leaf.Raw, cert.Leaf.Raw)
	})

	t.Run("a wrong password is an error", func(t *testing.T) {
		_, err := LoadCertificate(data, "wrong")
		assert.Error(t, err)
	})
}
