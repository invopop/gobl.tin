package gobltin

import "fmt"

// NetworkError represents an error that occurs during network operations.
type NetworkError struct {
	Msg string
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("Network error, %s", e.Msg)
}

// InvalidTaxIDError represents an error due to an invalid tax ID.
type InvalidTaxIDError struct {
	Msg string
}

func (e *InvalidTaxIDError) Error() string {
	return fmt.Sprintf("Tax ID Invalid, %s", e.Msg)
}
