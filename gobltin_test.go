package gobltin

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl.tin/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTinNumber(t *testing.T) {
	examples, err := test.GetDataGlob("*.json")
	require.NoError(t, err)

	for _, example := range examples {
		env, err := test.LoadTestEnvelope(example)
		require.NoError(t, err)

		tin, err := NewTinNumber(env)

		expectedTin, expectedErr, expectedValid := getExpectedResult(example)

		if expectedErr != nil {
			assert.Error(t, err)
			assert.Equal(t, expectedErr.Error(), err.Error())
		} else {
			assert.NoError(t, err)
			assert.Equal(t, expectedTin, tin)
			response, _ := tin.Lookup()
			assert.Equal(t, response.Valid, expectedValid)
		}
	}

}

// getExpectedResult returns the expected result for a given test file
func getExpectedResult(filePath string) (*TinNumber, error, bool) {
	// Here we define the expected result for the files in test/data
	fileName := filepath.Base(filePath)
	switch fileName {
	case "invoice-valid.json":
		return &TinNumber{CountryCode: "DE", TinNumber: "282741168"}, nil, true
	case "empty.json":
		return nil, errors.New("invalid document type"), false
	case "invoice-no-customer.json":
		return nil, errors.New("no customer found"), false
	case "invoice-no-taxid.json":
		return nil, errors.New("no tax ID found"), false
	case "invoice-no-country.json":
		return nil, errors.New("no country code found"), false
	case "invoice-no-code.json":
		return nil, errors.New("no tax ID code found"), false
	default:
		return nil, nil, false
	}
}
