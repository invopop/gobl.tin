package tin

import (
	"context"
	"time"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// ABOUT: a Verifier answers about one identifier; the Client picks a verifier
// per identifier and builds the report.

// Verifier is one register client.
type Verifier interface {
	// Source names the register, for example "vies".
	Source() cbc.Key

	// Supports reports whether the register covers the identifier.
	Supports(id Identifier) bool

	// Verify asks the register. An error means it could not answer.
	Verify(ctx context.Context, req Request) (*Answer, error)
}

// PartyKeyer is a Verifier whose answer also depends on party data.
type PartyKeyer interface {
	// PartyKey returns that data, normalized, or "" when none is used.
	PartyKey(req Request) string
}

// Request is what a Verifier receives.
type Request struct {
	Identifier Identifier

	// Party is read only, and nil for a single identifier.
	Party *org.Party
}

// Answer is what a register says about one identifier.
type Answer struct {
	// Status is valid or invalid; anything else counts as a failure.
	Status Status `json:"status"`

	Record *Record `json:"record,omitempty"`

	// TaxID as the register echoes it.
	TaxID *tax.Identity `json:"tax_id,omitempty"`

	// Zero means now.
	CheckedAt time.Time `json:"checked_at,omitzero"`

	// Reason for an invalid answer, readable by a user.
	Reason string `json:"reason,omitempty"`
}

// Status is the outcome of one check.
type Status string

// Check statuses.
const (
	StatusValid   Status = "valid"
	StatusInvalid Status = "invalid"

	// No verifier covers the identifier.
	StatusUnsupported Status = "unsupported"

	// The register could not answer; see Failure.
	StatusUnverified Status = "unverified"
)

// Check is the outcome for one identifier of a party.
type Check struct {
	// JSON Pointer of the identifier in the party: "/tax_id".
	Path string `json:"path,omitempty"`

	// Normalized, as the register echoes it when it does.
	TaxID *tax.Identity `json:"tax_id,omitempty"`

	Status Status `json:"status"`

	// Verifier that answered.
	Source cbc.Key `json:"source,omitempty"`

	CheckedAt  time.Time   `json:"checked_at,omitzero"`
	Record     *Record     `json:"record,omitempty"`
	Mismatches []*Mismatch `json:"mismatches,omitempty"`

	// Reason for an invalid answer, when the register gives one.
	Reason string `json:"reason,omitempty"`

	// Failure message when unverified.
	Failure string `json:"failure,omitempty"`

	err error
}

// Err returns the register failure behind an unverified check, or nil.
func (c *Check) Err() error {
	return c.err
}

func (c *Check) fail(err error) {
	c.Status = StatusUnverified
	c.err = err
	c.Failure = ""
	if err != nil {
		c.Failure = err.Error()
	}
}

// Record is what the register holds for an identifier.
type Record struct {
	Name string `json:"name,omitempty"`

	// The register compared the party's name and accepted it, so no name
	// mismatch is reported.
	NameConfirmed bool `json:"name_confirmed,omitempty"`

	// Structured address only.
	Address *org.Address `json:"address,omitempty"`

	// Standing of the party. It does not change Check.Status.
	Status RecordStatus `json:"status,omitempty"`
}

// RecordStatus is the standing of a party in its register.
type RecordStatus string

// Record statuses.
const (
	RecordStatusActive    RecordStatus = "active"
	RecordStatusInactive  RecordStatus = "inactive"
	RecordStatusDissolved RecordStatus = "dissolved"

	// The register identifies the party, but the identifier has no legal
	// effect.
	RecordStatusRevoked RecordStatus = "revoked"
)

// MismatchField names the kind of party field a mismatch is about.
type MismatchField string

// Mismatch fields.
const (
	MismatchName  MismatchField = "name"
	MismatchTaxID MismatchField = "tax_id"
)

// Mismatch is one field where the party disagrees with the record.
type Mismatch struct {
	Field MismatchField `json:"field"`

	// JSON Pointer of the field in the party, such as "/name".
	Path string `json:"path"`

	Document string `json:"document"`
	Register string `json:"register"`
}

// Report is the outcome for one party: one Check per identifier.
type Report struct {
	Checks []*Check `json:"checks"`
}

// Valid reports whether at least one check is valid and none is invalid or
// unverified.
func (r *Report) Valid() bool {
	if r == nil {
		return false
	}
	verified := false
	for _, c := range r.Checks {
		if c == nil {
			continue
		}
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
