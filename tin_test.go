package tin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeVerifier answers from a table keyed by identifier code.
type fakeVerifier struct {
	source   cbc.Key
	supports func(Identifier) bool
	checks   map[cbc.Code]*Check
	errs     map[cbc.Code]error
	calls    []Identifier
}

func (f *fakeVerifier) Source() cbc.Key { return f.source }

func (f *fakeVerifier) Supports(id Identifier) bool {
	if f.supports == nil {
		return true
	}
	return f.supports(id)
}

func (f *fakeVerifier) Verify(_ context.Context, id Identifier) (*Check, error) {
	f.calls = append(f.calls, id)
	if err, ok := f.errs[id.Code]; ok {
		return nil, err
	}
	if c, ok := f.checks[id.Code]; ok {
		cp := *c
		return &cp, nil
	}
	return &Check{Status: StatusInvalid, Source: f.source, CheckedAt: time.Now()}, nil
}

// taxOnly restricts a fake verifier to tax identities of the given countries.
func taxOnly(countries ...l10n.TaxCountryCode) func(Identifier) bool {
	return func(id Identifier) bool {
		return id.Type == "" && id.Key == "" && id.Country.In(countries...)
	}
}

// typeOnly restricts a fake verifier to identities of the given type.
func typeOnly(typ cbc.Code) func(Identifier) bool {
	return func(id Identifier) bool { return id.Type == typ }
}

func valid(source cbc.Key, rec *Record) *Check {
	return &Check{Status: StatusValid, Source: source, CheckedAt: time.Now(), Record: rec}
}

func TestVerify(t *testing.T) {
	ctx := context.Background()

	t.Run("nil party is an input error", func(t *testing.T) {
		_, err := New().Verify(ctx, nil)
		assert.ErrorIs(t, err, ErrInput)
	})

	t.Run("party without identifiers yields an empty report", func(t *testing.T) {
		report, err := New().Verify(ctx, &org.Party{Name: "Acme"})
		require.NoError(t, err)
		assert.Empty(t, report.Checks)
		assert.False(t, report.Valid())
	})

	t.Run("walks the tax id and identities in order", func(t *testing.T) {
		fake := &fakeVerifier{
			source:   "vies",
			supports: taxOnly("DE"),
			checks: map[cbc.Code]*Check{
				"282741168": valid("vies", &Record{Name: "ACME TRADING GMBH"}),
			},
		}
		party := &org.Party{
			Name:       "Acme Trading",
			TaxID:      &tax.Identity{Country: "DE", Code: "282741168"},
			Identities: []*org.Identity{{Country: "DE", Type: "HRB", Code: "12345"}},
		}
		report, err := New(fake).Verify(ctx, party)
		require.NoError(t, err)
		require.Len(t, report.Checks, 2)

		c := report.Checks[0]
		assert.Equal(t, "/tax_id", c.Path)
		assert.Equal(t, StatusValid, c.Status)
		assert.Equal(t, cbc.Key("vies"), c.Source)
		require.NotNil(t, c.TaxID)
		assert.Equal(t, "282741168", c.TaxID.Code.String())
		require.Len(t, c.Mismatches, 1)
		assert.Equal(t, &Mismatch{Field: MismatchName, Path: "/name", Document: "Acme Trading", Register: "ACME TRADING GMBH"}, c.Mismatches[0])

		u := report.Checks[1]
		assert.Equal(t, "/identities/0", u.Path)
		assert.Equal(t, StatusUnsupported, u.Status)
		assert.Empty(t, u.Source)
		assert.True(t, u.CheckedAt.IsZero())
		require.NotNil(t, u.Identity)
		assert.Equal(t, "HRB", u.Identity.Type.String())

		assert.True(t, report.Valid())
		require.Len(t, fake.calls, 1)
		assert.Equal(t, "/tax_id", fake.calls[0].Path)
	})

	t.Run("normalizes before calling the verifier and keeps the party", func(t *testing.T) {
		fake := &fakeVerifier{source: "vies", checks: map[cbc.Code]*Check{"282741168": valid("vies", nil)}}
		party := &org.Party{TaxID: &tax.Identity{Country: "DE", Code: "de 282-741-168"}}
		report, err := New(fake).Verify(ctx, party)
		require.NoError(t, err)
		assert.Equal(t, StatusValid, report.Checks[0].Status)
		assert.Equal(t, "282741168", fake.calls[0].Code.String())
		assert.Equal(t, "de 282-741-168", party.TaxID.Code.String())
	})

	t.Run("invalid is a status, not an error", func(t *testing.T) {
		fake := &fakeVerifier{source: "vies"}
		party := &org.Party{TaxID: &tax.Identity{Country: "DE", Code: "000000000"}}
		report, err := New(fake).Verify(ctx, party)
		require.NoError(t, err)
		assert.Equal(t, StatusInvalid, report.Checks[0].Status)
		assert.False(t, report.Valid())
	})

	t.Run("register failure marks the check unverified and continues", func(t *testing.T) {
		fake := &fakeVerifier{
			source: "vies",
			errs:   map[cbc.Code]error{"282741168": ErrServer.WithCode("500").WithMessage("MS_UNAVAILABLE")},
			checks: map[cbc.Code]*Check{"12345": valid("vies", nil)},
		}
		party := &org.Party{
			TaxID:      &tax.Identity{Country: "DE", Code: "282741168"},
			Identities: []*org.Identity{{Country: "DE", Type: "HRB", Code: "12345"}},
		}
		report, err := New(fake).Verify(ctx, party)
		require.NoError(t, err)
		require.Len(t, report.Checks, 2)

		c := report.Checks[0]
		assert.Equal(t, StatusUnverified, c.Status)
		assert.Equal(t, cbc.Key("vies"), c.Source)
		assert.Equal(t, "server: 500: MS_UNAVAILABLE", c.Error)
		assert.True(t, errors.Is(c.Err(), ErrServer))
		require.NotNil(t, c.TaxID)

		assert.Equal(t, StatusValid, report.Checks[1].Status)
		assert.False(t, report.Valid())
	})

	t.Run("rate limit is reachable through Err", func(t *testing.T) {
		fake := &fakeVerifier{source: "vies", errs: map[cbc.Code]error{"282741168": &RateLimitedError{RetryAfter: 30 * time.Second}}}
		report, err := New(fake).Verify(ctx, &org.Party{TaxID: &tax.Identity{Country: "DE", Code: "282741168"}})
		require.NoError(t, err)
		var rl *RateLimitedError
		require.True(t, errors.As(report.Checks[0].Err(), &rl))
		assert.Equal(t, 30*time.Second, rl.RetryAfter)
	})

	t.Run("first supporting verifier wins", func(t *testing.T) {
		first := &fakeVerifier{source: "vies", supports: taxOnly("DE"), checks: map[cbc.Code]*Check{"282741168": valid("vies", nil)}}
		second := &fakeVerifier{source: "other", checks: map[cbc.Code]*Check{"282741168": valid("other", nil), "12345": valid("other", nil)}}
		party := &org.Party{
			TaxID:      &tax.Identity{Country: "DE", Code: "282741168"},
			Identities: []*org.Identity{{Country: "DE", Type: "HRB", Code: "12345"}},
		}
		report, err := New(first, second).Verify(ctx, party)
		require.NoError(t, err)
		assert.Equal(t, cbc.Key("vies"), report.Checks[0].Source)
		assert.Equal(t, cbc.Key("other"), report.Checks[1].Source)
		assert.Len(t, first.calls, 1)
		assert.Len(t, second.calls, 1)
	})

	t.Run("identity mismatches", func(t *testing.T) {
		reg := &fakeVerifier{
			source:   "companies",
			supports: typeOnly("CRN"),
			checks: map[cbc.Code]*Check{
				"00445790": valid("companies", &Record{
					Name: "ACME LTD",
					Identities: []*org.Identity{
						{Country: "GB", Type: "CRN", Code: "00445790"},
						{Country: "GB", Type: "UTR", Code: "1234567890"},
						{Country: "GB", Type: "EORI", Code: "GB123"},
					},
				}),
			},
		}
		party := &org.Party{
			Name:  "Acme Ltd",
			TaxID: &tax.Identity{Country: "GB", Code: "123456789"},
			Identities: []*org.Identity{
				{Country: "GB", Type: "CRN", Code: "00445790"},
				{Country: "GB", Type: "UTR", Code: "0000000000"},
			},
		}
		report, err := New(reg).Verify(ctx, party)
		require.NoError(t, err)
		require.Len(t, report.Checks, 3)
		assert.Equal(t, StatusUnsupported, report.Checks[0].Status, "GB is not covered by VIES")
		c := report.Checks[1]
		assert.Equal(t, StatusValid, c.Status)
		require.Len(t, c.Mismatches, 1)
		assert.Equal(t, &Mismatch{Field: MismatchIdentity, Path: "/identities/1/code", Document: "0000000000", Register: "1234567890"}, c.Mismatches[0])
		assert.Equal(t, StatusUnsupported, report.Checks[2].Status)
		assert.True(t, report.Valid())
	})

	t.Run("tax id echo mismatch", func(t *testing.T) {
		echo := valid("vies", nil)
		echo.TaxID = &tax.Identity{Country: "DE", Code: "999999999"}
		fake := &fakeVerifier{source: "vies", checks: map[cbc.Code]*Check{"282741168": echo}}
		report, err := New(fake).Verify(ctx, &org.Party{TaxID: &tax.Identity{Country: "DE", Code: "282741168"}})
		require.NoError(t, err)
		c := report.Checks[0]
		require.Len(t, c.Mismatches, 1)
		assert.Equal(t, &Mismatch{Field: MismatchTaxID, Path: "/tax_id/code", Document: "DE282741168", Register: "DE999999999"}, c.Mismatches[0])
	})

	t.Run("matching name is not a mismatch", func(t *testing.T) {
		fake := &fakeVerifier{source: "vies", checks: map[cbc.Code]*Check{"282741168": valid("vies", &Record{Name: "ACME GMBH"})}}
		report, err := New(fake).Verify(ctx, &org.Party{Name: "Acme GmbH", TaxID: &tax.Identity{Country: "DE", Code: "282741168"}})
		require.NoError(t, err)
		assert.Empty(t, report.Checks[0].Mismatches)
	})

	t.Run("empty party name is not a mismatch", func(t *testing.T) {
		fake := &fakeVerifier{source: "vies", checks: map[cbc.Code]*Check{"282741168": valid("vies", &Record{Name: "ACME GMBH"})}}
		report, err := New(fake).Verify(ctx, &org.Party{TaxID: &tax.Identity{Country: "DE", Code: "282741168"}})
		require.NoError(t, err)
		assert.Empty(t, report.Checks[0].Mismatches)
	})
}

func TestVerifyTaxID(t *testing.T) {
	ctx := context.Background()
	fake := &fakeVerifier{source: "vies", supports: taxOnly("DE"), checks: map[cbc.Code]*Check{"282741168": valid("vies", &Record{Name: "ACME GMBH"})}}
	c := New(fake)

	t.Run("valid", func(t *testing.T) {
		check, err := c.VerifyTaxID(ctx, &tax.Identity{Country: "DE", Code: "282741168"})
		require.NoError(t, err)
		assert.Equal(t, "/tax_id", check.Path)
		assert.Equal(t, StatusValid, check.Status)
		assert.Empty(t, check.Mismatches, "no party to compare the name with")
		assert.Equal(t, "ACME GMBH", check.Record.Name)
	})

	t.Run("unsupported is a check", func(t *testing.T) {
		check, err := c.VerifyTaxID(ctx, &tax.Identity{Country: "US", Code: "123456789"})
		require.NoError(t, err)
		assert.Equal(t, StatusUnsupported, check.Status)
		require.NotNil(t, check.TaxID)
	})

	t.Run("nil and empty code are input errors", func(t *testing.T) {
		_, err := c.VerifyTaxID(ctx, nil)
		assert.ErrorIs(t, err, ErrInput)
		_, err = c.VerifyTaxID(ctx, &tax.Identity{Country: "DE"})
		assert.ErrorIs(t, err, ErrInput)
	})
}

func TestVerifyIdentity(t *testing.T) {
	ctx := context.Background()
	reg := &fakeVerifier{source: "companies", supports: typeOnly("CRN"), checks: map[cbc.Code]*Check{"00445790": valid("companies", nil)}}
	c := New(reg)

	t.Run("valid", func(t *testing.T) {
		check, err := c.VerifyIdentity(ctx, &org.Identity{Country: "GB", Type: "CRN", Code: "00445790"})
		require.NoError(t, err)
		assert.Equal(t, "/identities/0", check.Path)
		assert.Equal(t, StatusValid, check.Status)
		require.NotNil(t, check.Identity)
		assert.Nil(t, check.TaxID)
	})

	t.Run("unsupported is a check", func(t *testing.T) {
		check, err := c.VerifyIdentity(ctx, &org.Identity{Country: "DE", Type: "HRB", Code: "12345"})
		require.NoError(t, err)
		assert.Equal(t, StatusUnsupported, check.Status)
	})

	t.Run("nil and empty code are input errors", func(t *testing.T) {
		_, err := c.VerifyIdentity(ctx, nil)
		assert.ErrorIs(t, err, ErrInput)
		_, err = c.VerifyIdentity(ctx, &org.Identity{Type: "CRN"})
		assert.ErrorIs(t, err, ErrInput)
	})
}

func TestNew(t *testing.T) {
	t.Run("no verifiers marks everything unsupported", func(t *testing.T) {
		c := New()
		report, err := c.Verify(context.Background(), &org.Party{TaxID: &tax.Identity{Country: "DE", Code: "282741168"}})
		require.NoError(t, err)
		require.Len(t, report.Checks, 1)
		assert.Equal(t, StatusUnsupported, report.Checks[0].Status)
		assert.False(t, report.Valid())
	})

	t.Run("nil verifiers are skipped", func(t *testing.T) {
		fake := &fakeVerifier{source: "vies"}
		c := New(nil, fake, nil)
		require.Len(t, c.verifiers, 1)
		assert.Same(t, fake, c.verifiers[0])
	})

	t.Run("argument order is routing order", func(t *testing.T) {
		first := &fakeVerifier{source: "vies", checks: map[cbc.Code]*Check{"282741168": valid("vies", nil)}}
		second := &fakeVerifier{source: "vies", checks: map[cbc.Code]*Check{"282741168": valid("vies", nil)}}
		c := New(first, second)
		require.Len(t, c.verifiers, 2, "a shared source keeps both")
		_, err := c.VerifyTaxID(context.Background(), &tax.Identity{Country: "DE", Code: "282741168"})
		require.NoError(t, err)
		assert.Len(t, first.calls, 1)
		assert.Empty(t, second.calls)
	})
}

func TestSupports(t *testing.T) {
	c := New(
		&fakeVerifier{source: "vies", supports: taxOnly("ES", "EL", "XI")},
		&fakeVerifier{source: "companies", supports: typeOnly("CRN")},
	)
	assert.True(t, c.SupportsTaxID(&tax.Identity{Country: "ES", Code: "B85905495"}))
	assert.True(t, c.SupportsTaxID(&tax.Identity{Country: "XI", Code: "123456789"}))
	assert.False(t, c.SupportsTaxID(&tax.Identity{Country: "US", Code: "123456789"}))
	assert.False(t, c.SupportsTaxID(nil))

	assert.True(t, c.SupportsIdentity(&org.Identity{Country: "GB", Type: "CRN", Code: "00445790"}))
	assert.False(t, c.SupportsIdentity(&org.Identity{Country: "DE", Type: "HRB", Code: "12345"}))
	assert.False(t, c.SupportsIdentity(nil))

	assert.False(t, New().SupportsTaxID(&tax.Identity{Country: "ES", Code: "B85905495"}))
}
