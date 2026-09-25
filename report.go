package tin

import (
	"context"
	"time"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// ABOUT: A Verifier is one register client. It answers about one identifier
// at a time and receives the party only as read-only context, for registers
// that need more than the code, such as a name. The Client walks the party,
// picks a verifier per identifier, and turns the answers into a report with
// mismatches. Errors from Verify are reserved for the cases where the register
// could not answer; an invalid identifier is a Check with StatusInvalid and a
// nil error. Verifiers live in their own packages, such as vies, and import
// this one.

// Verifier is one register client.
type Verifier interface {
	// Source names the register, for example "vies".
	Source() cbc.Key

	// Supports reports whether the register covers the identifier.
	Supports(id Identifier) bool

	// Verify asks the register about the request's identifier. ErrInput
	// covers input the register rejects or lacks, ErrServer, ErrNetwork and
	// RateLimitedError cover a register that cannot answer.
	Verify(ctx context.Context, req Request) (*Answer, error)
}

// Request is what a Verifier receives: the identifier to answer for and the
// party it belongs to.
type Request struct {
	// Identifier is the identifier to verify.
	Identifier Identifier

	// Party is the party the identifier belongs to. It is read only, and nil
	// when a single identifier is verified without a party.
	Party *org.Party
}

// Answer is what a register says about one identifier. The Client turns it
// into a Check.
type Answer struct {
	// Status is valid or invalid. Any other status, including empty, makes
	// the check unverified.
	Status Status

	// Record is what the register holds for the identifier, when disclosed.
	Record *Record

	// TaxID is the tax identity as the register echoes it, when it does.
	TaxID *tax.Identity

	// CheckedAt is when the register answered. Zero means now.
	CheckedAt time.Time
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
	// register. The Check carries the failure in Failure.
	StatusUnverified Status = "unverified"
)

// Check is the outcome for one identifier of a party.
type Check struct {
	// Path is the JSON Pointer of the identifier in the party: "/tax_id" or
	// "/identities/N".
	Path string `json:"path"`

	// TaxID is the normalized tax identity when the identifier is the
	// party's tax_id. When the verifier's answer echoes one, the Client sets
	// it from that echo, normalized.
	TaxID *tax.Identity `json:"tax_id,omitempty"`

	// Identity is the normalized identity when the identifier is one of the
	// party's identities.
	Identity *org.Identity `json:"identity,omitempty"`

	// Status is the outcome of the check.
	Status Status `json:"status"`

	// Source names the verifier that answered. Empty when unsupported.
	Source cbc.Key `json:"source,omitempty"`

	// CheckedAt is when the register answered. Absent when unsupported or
	// unverified.
	CheckedAt time.Time `json:"checked_at,omitzero"`

	// Record is what the register holds for the identifier, when disclosed.
	Record *Record `json:"record,omitempty"`

	// Mismatches lists the fields where the party disagrees with the record.
	Mismatches []*Mismatch `json:"mismatches,omitempty"`

	// Failure is the register failure message when Status is unverified.
	Failure string `json:"failure,omitempty"`

	err error
}

// Err returns the register failure behind an unverified check, or nil.
func (c *Check) Err() error {
	return c.err
}

// fail marks the check as unverified with the register failure.
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
	// Name is the name the register holds for the party.
	Name string `json:"name,omitempty"`

	// Identities lists further identifiers the register holds.
	Identities []*org.Identity `json:"identities,omitempty"`

	// Addresses lists the addresses the register holds in structured form.
	Addresses []*org.Address `json:"addresses,omitempty"`

	// Status is the standing of the party in the register. It does not
	// change Check.Status: a recognised but dissolved company is valid with
	// Status dissolved, and the consumer decides.
	Status RecordStatus `json:"status,omitempty"`
}

// RecordStatus is the standing of a party in its register. Verifiers map
// register values onto this vocabulary and leave it empty for values they
// do not know.
type RecordStatus string

// Record statuses.
const (
	// RecordStatusActive means the register holds the party as active.
	RecordStatusActive RecordStatus = "active"

	// RecordStatusInactive means the register holds the party as inactive
	// or suspended.
	RecordStatusInactive RecordStatus = "inactive"

	// RecordStatusDissolved means the register holds the party as dissolved
	// or struck off.
	RecordStatusDissolved RecordStatus = "dissolved"
)

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

// Report is the outcome for one party: one Check per identifier, in
// document order.
type Report struct {
	// Checks lists one outcome per identifier of the party.
	Checks []*Check `json:"checks"`
}

// Valid reports whether every check that a verifier answered is valid. A
// report with an unverified check is not valid. A report where no verifier
// covered any identifier is not valid either: no identifier is verified.
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
