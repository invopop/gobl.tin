package gobltin

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl.tin/test"
	"github.com/invopop/gobl/tax"
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

		expectedTin, expectedValid, expectedErr := getExpectedResult(example)

		if expectedErr != nil {
			assert.Error(t, err)
			assert.Equal(t, expectedErr.Error(), err.Error())
		} else {
			assert.NoError(t, err)
			assert.Equal(t, expectedTin, tin)
			ctx := context.Background()
			response, _ := Lookup(ctx, tin)
			assert.Equal(t, expectedValid, response.Valid)
		}
	}

}

// getExpectedResult returns the expected result for a given test file
func getExpectedResult(filePath string) (*tax.Identity, bool, error) {
	// Here we define the expected result for the files in test/data
	fileName := filepath.Base(filePath)
	switch fileName {
	case "invoice-valid.json":
		return &tax.Identity{Country: "DE", Code: "282741168"}, true, nil
	case "empty.json":
		return nil, false, errors.New("invalid document type")
	case "invoice-no-customer.json":
		return nil, false, errors.New("no customer found")
	case "invoice-no-taxid.json":
		return nil, false, errors.New("no tax ID found")
	case "invoice-no-country.json":
		return nil, false, errors.New("no country code found")
	case "invoice-no-code.json":
		return nil, false, errors.New("no tax ID code found")
	default:
		return nil, false, nil
	}
}
