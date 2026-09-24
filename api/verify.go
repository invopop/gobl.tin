package api

import (
	"context"
	"time"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// ABOUT: A Verifier is one register client. It answers about one identifier
// at a time and knows nothing about the party. The Client walks the party,
// picks a verifier per identifier, and turns the answers into a report with
// mismatches. Errors from Verify are reserved for the cases where the register
// could not answer; an invalid identifier is a Check with StatusInvalid and a
// nil error.

// Verifier is one register client.
type Verifier interface {
	// Source names the register, for example "vies".
	Source() cbc.Key

	// Supports reports whether the register covers the identifier.
	Supports(id Identifier) bool

	// Verify asks the register about the identifier. ErrInput covers input
	// the register rejects, ErrServer, ErrNetwork and RateLimitedError cover
	// a register that cannot answer.
	Verify(ctx context.Context, id Identifier) (*Check, error)
}

// Status is the outcome of one check.
type Status string

// Check statuses.
const (
	// StatusValid means the register recognises the identifier.
	StatusValid Status = "valid"

	// StatusInvalid means the register does not recognise the identifier.
	StatusInvalid Status = "invalid"

	// StatusUnsupported means no verifier covers the identifier.
	StatusUnsupported Status = "unsupported"

	// StatusUnverified means the verifier could not get an answer from the
	// register. The Check carries the failure in Error.
	StatusUnverified Status = "unverified"
)

// Check is the outcome for one identifier of a party.
type Check struct {
	// Path is the JSON Pointer of the identifier in the party: "/tax_id" or
	// "/identities/N".
	Path string `json:"path"`

	// TaxID is the normalized tax identity when the identifier is the
	// party's tax_id. A verifier sets it to the identity the register echoes.
	TaxID *tax.Identity `json:"tax_id,omitempty"`

	// Identity is the normalized identity when the identifier is one of the
	// party's identities.
	Identity *org.Identity `json:"identity,omitempty"`

	// Status is the outcome of the check.
	Status Status `json:"status"`

	// Source names the verifier that answered. Empty when unsupported.
	Source cbc.Key `json:"source,omitempty"`

	// CheckedAt is when the register answered. Zero when unsupported.
	CheckedAt time.Time `json:"checked_at,omitzero"`

	// Record is what the register holds for the identifier, when disclosed.
	Record *Record `json:"record,omitempty"`

	// Mismatches lists the fields where the party disagrees with the record.
	Mismatches []*Mismatch `json:"mismatches,omitempty"`

	// Error is the register failure message when Status is unverified.
	Error string `json:"error,omitempty"`

	err error
}

// Err returns the register failure behind an unverified check, or nil.
func (c *Check) Err() error {
	return c.err
}

// Fail marks the check as unverified with the register failure.
func (c *Check) Fail(err error) {
	c.Status = StatusUnverified
	c.err = err
	c.Error = ""
	if err != nil {
		c.Error = err.Error()
	}
}

// Record is what the register holds for an identifier.
type Record struct {
	// Name is the name the register holds for the party.
	Name string `json:"name,omitempty"`

	// Identities lists further identifiers the register holds.
	Identities []*org.Identity `json:"identities,omitempty"`

	// Addresses lists the addresses the register holds in structured form.
	Addresses []*org.Address `json:"addresses,omitempty"`

	// Status is the standing of the party in the register.
	Status cbc.Key `json:"status,omitempty"`
}

// MismatchField names the kind of party field a mismatch is about.
type MismatchField string

// Mismatch fields.
const (
	// MismatchName is a disagreement on the party's name.
	MismatchName MismatchField = "name"

	// MismatchTaxID is a disagreement on the party's tax identity code.
	MismatchTaxID MismatchField = "tax_id"

	// MismatchIdentity is a disagreement on the code of one identity.
	MismatchIdentity MismatchField = "identity"

	// MismatchAddress is a disagreement on one address field.
	MismatchAddress MismatchField = "address"
)

// Mismatch is one field where the party disagrees with the record. Go
// consumers switch on Field; Path locates the value within arrays.
type Mismatch struct {
	// Field is the kind of party field the mismatch is about.
	Field MismatchField `json:"field"`

	// Path is the JSON Pointer of the field in the party, for example
	// "/name" or "/identities/1/code".
	Path string `json:"path"`

	// Document is the value in the party.
	Document string `json:"document"`

	// Register is the value in the record.
	Register string `json:"register"`
}

// Verification is the report for one party: one Check per identifier, in
// document order.
type Verification struct {
	// Checks lists one outcome per identifier of the party.
	Checks []*Check `json:"checks"`

	party *org.Party
}

// NewVerification starts a report for the party. Patch reads the party, so a
// report built without one cannot produce a patch.
func NewVerification(party *org.Party) *Verification {
	return &Verification{party: party}
}

// Valid reports whether every check that a verifier answered is valid. A
// report with an unverified check is not valid. A report where no verifier
// covered any identifier is not valid either: no identifier is verified.
func (v *Verification) Valid() bool {
	if v == nil {
		return false
	}
	verified := false
	for _, c := range v.Checks {
		switch c.Status {
		case StatusValid:
			verified = true
		case StatusUnsupported:
		default:
			return false
		}
	}
	return verified
}
