package api

import (
	"encoding/json"
	"reflect"

	"github.com/invopop/gobl/org"
)

// ABOUT: A patch is an RFC 7396 JSON merge patch on the party. It carries
// only what the policy permits. The defaults are conservative: this code
// writes into documents that consumers send to their own customers, so it
// fills gaps and never resolves a conflict by overwriting. A merge patch
// replaces arrays whole, so identities and addresses always appear as the
// full array: the party's entries followed by the additions.
//
// Field precedence: checks are read in report order and the first valid
// record that has a value for a field wins. Records behind invalid,
// unverified or unsupported checks are never read.

// NamePolicy decides when the register's name is written.
type NamePolicy string

// Name policies. The empty value means NamePolicyFill.
const (
	// NamePolicyFill writes the register's name only when the party has
	// none. It is the default: registers store names upper cased and with
	// the full legal form, so a rewrite would touch every party.
	NamePolicyFill NamePolicy = "fill"

	// NamePolicyPreferRegister writes the register's name whenever the
	// register has one and it differs from the party's name, including a
	// difference of case or punctuation only.
	NamePolicyPreferRegister NamePolicy = "prefer-register"

	// NamePolicyKeep never writes the name.
	NamePolicyKeep NamePolicy = "keep"
)

// IdentitiesPolicy decides when the register's identities are written.
type IdentitiesPolicy string

// Identities policies. The empty value means IdentitiesPolicyAddMissing.
const (
	// IdentitiesPolicyAddMissing appends record identities of a country,
	// key and type the party lacks. An identity of the same country, key
	// and type with a different code is a conflict and is never written.
	IdentitiesPolicyAddMissing IdentitiesPolicy = "add-missing"

	// IdentitiesPolicyKeep never writes identities.
	IdentitiesPolicyKeep IdentitiesPolicy = "keep"
)

// AddressPolicy decides when the register's addresses are written.
type AddressPolicy string

// Address policies. The empty value means AddressPolicyNone.
const (
	// AddressPolicyNone never writes addresses. It is the default: an
	// invoice address is usually the trading address, not the registered
	// office.
	AddressPolicyNone AddressPolicy = ""

	// AddressPolicyAppend adds record addresses the party lacks. Two
	// addresses are the same when their labels match, or when they are
	// equal field by field if neither has a label.
	AddressPolicyAppend AddressPolicy = "append"

	// AddressPolicyReplace makes the record's addresses the party's
	// addresses.
	AddressPolicyReplace AddressPolicy = "replace"
)

// Policy decides what a Patch may write. The zero value fills an empty name,
// adds missing identities and leaves addresses alone.
type Policy struct {
	Name       NamePolicy
	Identities IdentitiesPolicy
	Addresses  AddressPolicy
}

// Patch builds a JSON merge patch on the party under the policy. A report
// without a party, or an unknown policy value, is an input error. A patch
// with nothing to write is "{}".
func (v *Verification) Patch(p Policy) (json.RawMessage, error) {
	if v == nil || v.party == nil {
		return nil, ErrInput.WithMessage("report has no party")
	}
	patch := map[string]any{}

	name, err := v.patchName(p.Name)
	if err != nil {
		return nil, err
	}
	if name != "" {
		patch["name"] = name
	}

	ids, err := v.patchIdentities(p.Identities)
	if err != nil {
		return nil, err
	}
	if ids != nil {
		patch["identities"] = ids
	}

	addrs, err := v.patchAddresses(p.Addresses)
	if err != nil {
		return nil, err
	}
	if addrs != nil {
		patch["addresses"] = addrs
	}

	return json.Marshal(patch)
}

// patchName returns the name to write, or empty when the policy writes
// nothing.
func (v *Verification) patchName(p NamePolicy) (string, error) {
	name := v.recordName()
	switch p {
	case "", NamePolicyFill:
		if v.party.Name == "" {
			return name, nil
		}
		return "", nil
	case NamePolicyPreferRegister:
		if name != v.party.Name {
			return name, nil
		}
		return "", nil
	case NamePolicyKeep:
		return "", nil
	default:
		return "", ErrInput.WithMsgf("unknown name policy %q", p)
	}
}

// patchIdentities returns the full identities array to write, or nil when
// the policy writes nothing.
func (v *Verification) patchIdentities(p IdentitiesPolicy) ([]*org.Identity, error) {
	switch p {
	case "", IdentitiesPolicyAddMissing:
	case IdentitiesPolicyKeep:
		return nil, nil
	default:
		return nil, ErrInput.WithMsgf("unknown identities policy %q", p)
	}
	var additions []*org.Identity
	for _, r := range v.recordIdentities() {
		if r == nil || hasIdentityKind(v.party.Identities, r) {
			continue
		}
		cp := *r
		additions = append(additions, &cp)
	}
	if len(additions) == 0 {
		return nil, nil
	}
	out := make([]*org.Identity, 0, len(v.party.Identities)+len(additions))
	for _, id := range v.party.Identities {
		if id != nil {
			out = append(out, id)
		}
	}
	return append(out, additions...), nil
}

// patchAddresses returns the full addresses array to write, or nil when the
// policy writes nothing.
func (v *Verification) patchAddresses(p AddressPolicy) ([]*org.Address, error) {
	recorded := v.recordAddresses()
	switch p {
	case AddressPolicyNone:
		return nil, nil
	case AddressPolicyReplace:
		if len(recorded) == 0 {
			return nil, nil
		}
		return recorded, nil
	case AddressPolicyAppend:
	default:
		return nil, ErrInput.WithMsgf("unknown address policy %q", p)
	}
	var additions []*org.Address
	for _, r := range recorded {
		if r == nil || hasAddress(v.party.Addresses, r) {
			continue
		}
		additions = append(additions, r)
	}
	if len(additions) == 0 {
		return nil, nil
	}
	out := make([]*org.Address, 0, len(v.party.Addresses)+len(additions))
	for _, a := range v.party.Addresses {
		if a != nil {
			out = append(out, a)
		}
	}
	return append(out, additions...), nil
}

// recordName returns the name of the first valid record that has one.
func (v *Verification) recordName() string {
	for _, c := range v.validRecords() {
		if c.Name != "" {
			return c.Name
		}
	}
	return ""
}

// recordIdentities returns the identities of the first valid record that
// has any.
func (v *Verification) recordIdentities() []*org.Identity {
	for _, c := range v.validRecords() {
		if len(c.Identities) > 0 {
			return c.Identities
		}
	}
	return nil
}

// recordAddresses returns the addresses of the first valid record that has
// any.
func (v *Verification) recordAddresses() []*org.Address {
	for _, c := range v.validRecords() {
		if len(c.Addresses) > 0 {
			return c.Addresses
		}
	}
	return nil
}

// validRecords lists the records behind valid checks, in report order.
func (v *Verification) validRecords() []*Record {
	var out []*Record
	for _, c := range v.Checks {
		if c != nil && c.Status == StatusValid && c.Record != nil {
			out = append(out, c.Record)
		}
	}
	return out
}

// hasIdentityKind reports whether the set holds an identity of the same
// country, key and type as r, whatever its code.
func hasIdentityKind(set []*org.Identity, r *org.Identity) bool {
	for _, id := range set {
		if id != nil && id.Country == r.Country && id.Key == r.Key && id.Type == r.Type {
			return true
		}
	}
	return false
}

// hasAddress reports whether the set holds r: by label when r has one, else
// field by field.
func hasAddress(set []*org.Address, r *org.Address) bool {
	for _, a := range set {
		if a == nil {
			continue
		}
		if r.Label != "" {
			if a.Label == r.Label {
				return true
			}
			continue
		}
		if a.Label == "" && reflect.DeepEqual(a, r) {
			return true
		}
	}
	return false
}
