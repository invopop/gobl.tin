package gobltin

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl.tin/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupTin(t *testing.T) {
	examples, err := test.GetDataGlob("*.json")
	require.NoError(t, err)

	for _, example := range examples {
		env, err := test.LoadTestEnvelope(example)
		require.NoError(t, err)

		results, err := LookupTin(env, Both)

		expectedResult, expectedErr := getExpectedResult(example)

		if expectedErr != nil {
			assert.Error(t, err)
			assert.Equal(t, expectedErr.Error(), err.Error())
		} else {
			assert.NoError(t, err)
			assert.Equal(t, expectedResult, results)
		}
	}

}

// getExpectedResult returns the expected result for a given test file
func getExpectedResult(filePath string) ([]*PartyTinResponse, error) {
	// Here we define the expected result for the files in test/data
	fileName := filepath.Base(filePath)
	switch fileName {
	case "invoice-valid.json":
		return []*PartyTinResponse{
			{
				Party:   Customer,
				Valid:   true,
				Message: "customer: valid",
			},
			{
				Party:   Supplier,
				Valid:   false,
				Message: "supplier: Tax ID Invalid, tax ID not found in database",
			},
		}, nil
	case "empty.json":
		return nil, errors.New("invalid type *schema.Object")
	case "invoice-no-customer.json":
		return []*PartyTinResponse{
			{
				Party:   Customer,
				Valid:   false,
				Message: "no customer found",
			},
			{
				Party:   Supplier,
				Valid:   false,
				Message: "supplier: Tax ID Invalid, tax ID not found in database",
			},
		}, nil
	case "invoice-no-taxid.json":
		return []*PartyTinResponse{
			{
				Party:   Customer,
				Valid:   false,
				Message: "customer: Tax ID Invalid, no tax ID provided",
			},
			{
				Party:   Supplier,
				Valid:   false,
				Message: "supplier: Tax ID Invalid, tax ID not found in database",
			},
		}, nil
	default:
		return nil, errors.New("unexpected file")
	}
}
