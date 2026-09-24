# gobl.tin

Verify the identifiers of GOBL parties against the registers that issue them, read what the register holds, and patch the party.

Copyright [Invopop Ltd.](https://invopop.com) 2026. Released publicly under the [Apache License Version 2.0](LICENSE). For commercial licenses please contact the [dev team at invopop](mailto:dev@invopop.com). In order to accept contributions to this library we will require transferring copyrights to Invopop Ltd.

## Vocabulary

- A **party** is a GOBL `org.Party`.
- An **identifier** is the party's `tax_id` or one entry of its `identities`.
- A **verifier** is one register client. `vies.New()` is the built-in verifier.
- A **register** is the external authority a verifier asks, for example VIES.
- A **check** is the outcome for one identifier.
- A **report** is the `Report` for one party: one check per identifier.
- A **record** is what the register holds for an identifier.
- A **mismatch** is one field where the party disagrees with the record.
- A **policy** decides what a **patch** may write back into the party.

## Go package

### Verify a party

`tin.New` takes an ordered list of verifiers. For each identifier of a party, the `tax_id` first and then each identity in order, the first verifier that supports it answers. `Client.Verify` returns an error only for a nil party. Everything the registers say, including a register that cannot answer, is in the report.

```go
package main

import (
	"context"
	"fmt"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/vies"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

func main() {
	party := &org.Party{
		Name:  "Acme Trading",
		TaxID: &tax.Identity{Country: "DE", Code: "282741168"},
		Identities: []*org.Identity{
			{Country: "DE", Type: "HRB", Code: "12345"},
		},
	}

	c := tin.New(vies.New())
	report, err := c.Verify(context.Background(), party)
	if err != nil {
		panic(err)
	}

	for _, check := range report.Checks {
		fmt.Println(check.Path, check.Status, check.Source)
		for _, m := range check.Mismatches {
			fmt.Printf("  %s: document %q, register %q\n", m.Path, m.Document, m.Register)
		}
	}
	fmt.Println("valid:", report.Valid())
}
```

A client with no verifiers, `tin.New()`, is valid and marks every identifier `unsupported`. Nil verifiers are skipped. Two verifiers may share a `Source`; argument order still decides which one answers.

The client holds no cache, so every call reaches the register. Caching is the consuming application's decision. The client is safe for concurrent use when its verifiers are. VIES requests time out after 15 seconds by default.

Single identifiers have their own methods. Both return a check with an empty `Path`, because there is no party document to point into. An identifier that no verifier covers is a check with status `unsupported`, not an error:

```go
check, err := c.VerifyTaxID(ctx, &tax.Identity{Country: "DE", Code: "282741168"})
check, err := c.VerifyIdentity(ctx, &org.Identity{Country: "GB", Type: "CRN", Code: "00445790"})
```

Coverage is queryable without a request, over the client's verifiers:

```go
c.SupportsTaxID(&tax.Identity{Country: "ES", Code: "B85905495"})            // true with VIES
c.SupportsIdentity(&org.Identity{Country: "DE", Type: "HRB", Code: "12345"}) // false with VIES
```

### Configure VIES

```go
c := tin.New(vies.New(
	vies.WithTimeout(5*time.Second),
	vies.WithBaseURL(url),
	vies.WithHTTPClient(httpClient),
))
```

### Write a verifier

A verifier implements `tin.Verifier`:

```go
type Verifier interface {
	Source() cbc.Key
	Supports(id Identifier) bool
	Verify(ctx context.Context, id Identifier) (*Answer, error)
}
```

`Identifier` is the input a verifier receives: the path of the identifier in the party, whether it is the tax identity, its country, key, type and normalized code. Consumers never construct one; the client builds them from GOBL types.

`Answer` is only what the register knows: `Status` (`valid` or `invalid`), the `Record` when disclosed, the `TaxID` the register echoes, and `CheckedAt` (zero means now). The client builds the check around it: path, identifier, source, mismatches. A verifier returns an error only when the register cannot answer. An invalid identifier is an answer with status `invalid` and a nil error. Any status other than `valid` or `invalid`, including empty, makes the check `unverified`.

## Report

A report has one check per identifier, in document order. A check carries:

- `path`: the JSON Pointer ([RFC 6901](https://www.rfc-editor.org/rfc/rfc6901)) of the identifier in the party, `/tax_id` or `/identities/N`.
- `tax_id` or `identity`: the normalized identifier. For a `tax_id` check, VIES sets it to the identity the register echoes.
- `status`: `valid`, `invalid`, `unsupported` or `unverified`.
- `source`: the verifier that answered, for example `vies`. Empty when unsupported.
- `checked_at`: when the register answered. Absent when unsupported or unverified.
- `record`: what the register holds, when disclosed. VIES discloses the name for most member states; some always mask it.
- `mismatches`: fields where the party disagrees with the record. Each carries a typed `field` (`name`, `tax_id` or `identity`), the JSON Pointer `path` of the value (`/name`, `/tax_id/code`, `/tax_id/country`, `/identities/N/code`), the `document` value and the `register` value. The client compares the name with `NameMatches`, which folds case, punctuation and whitespace, and compares identifiers by normalized code.
- `failure`: the register failure message, only when the status is `unverified`. The Go error is available through `Check.Err()`.

Every path is a JSON Pointer relative to the party document. A consumer that patches an invoice prefixes the party's location, `/customer` or `/supplier`. Go consumers match on the typed values, `Check.TaxID`, `Check.Identity` and `Mismatch.Field`, rather than parsing paths; the pointer is for JSON consumers and for locating a value within an array.

```json
{"checks":[{"path":"/tax_id","tax_id":{"country":"DE","code":"282741168"},"status":"valid","source":"vies","checked_at":"2026-09-24T09:12:00Z","record":{"name":"ACME TRADING GMBH"},"mismatches":[{"field":"name","path":"/name","document":"Acme Trading","register":"ACME TRADING GMBH"}]},{"path":"/identities/0","identity":{"country":"DE","type":"HRB","code":"12345"},"status":"unsupported"}]}
```

`Report.Valid()` is true when every check that a verifier answered is `valid`. A report with an `invalid` or `unverified` check is not valid. A report where every check is `unsupported` is not valid either: no identifier is verified.

### Record status

`Record.Status` is the standing of the party in its register: `active`, `inactive` or `dissolved`. Verifiers map register values onto this vocabulary and leave it empty for values they do not know. It does not change the check status: a recognised but dissolved company is `valid` with `Record.Status` `dissolved`, and the consumer decides. VIES has no standing and leaves it empty.

## Patch and Policy

`tin.Patch(party, report, policy)` builds an [RFC 7396](https://www.rfc-editor.org/rfc/rfc7396) JSON merge patch on the party. It contains only what the policy permits. A patch with nothing to write is `{}`. A nil party or report, or an unknown policy value, is `ErrInput`.

```go
patch, err := tin.Patch(party, report, tin.Policy{
	Name:       tin.NamePolicyPreferRegister,
	Identities: tin.IdentitiesPolicyAddMissing,
	Addresses:  tin.AddressPolicyAppend,
})
// patch: {"name":"ACME TRADING GMBH"}
```

Records are read in report order. The first valid record that has a value for a field wins. Records behind `invalid`, `unverified` or `unsupported` checks are never read.

Every policy field has a named default, and the empty string is an alias for it, so the zero `Policy` is safe.

| Field | Policy | Default | Effect |
| --- | --- | --- | --- |
| Name | `fill` | yes | Writes the register's name only when the party has none. Registers store names upper cased and with the full legal form, so a rewrite would touch every party. |
| Name | `prefer-register` | | Writes the register's name whenever the register has one and it differs from the party's, including a difference of case or punctuation only. |
| Name | `keep` | | Never writes the name. |
| Identities | `add-missing` | yes | Appends record identities of a kind (country, type and key) the party lacks. An identity of the same kind with a different code is a conflict and is never written. A record identity with neither type nor key is never written. |
| Identities | `keep` | | Never writes identities. |
| Addresses | `none` | yes | Never writes addresses. An invoice address is usually the trading address, not the registered office. |
| Addresses | `append` | | Adds record addresses the party lacks. Two addresses are the same when their labels match, or when their postal fields are equal if neither has a label. |
| Addresses | `replace` | | Makes the record's addresses the party's addresses. |

A merge patch replaces arrays whole, so `identities` and `addresses` always appear as the full array: the party's entries, in their positions, followed by the additions. Of two record identities of one kind, only the first is added. A patch never changes `tax_id` and never removes an identity or an address, except under `replace`. VIES returns no identities and no structured address, so a report from VIES alone patches at most the name.

## Errors

An invalid identifier is a status, not an error. A register that cannot answer is a check with status `unverified` that carries the failure; `Verify` still returns a nil error and the report continues with the next identifier. `Check.Err()` returns the Go error so that callers can match it:

- `ErrInput`: the input is malformed or incomplete. A nil party, an identifier without a code, or a request the register rejected as badly formed (HTTP 400).
- `ErrNetwork`: the request failed below HTTP. A dial error, a timeout, or a response that could not be decoded. It says nothing about the identifier.
- `ErrServer`: the register answered with an unexpected status, for example a 500. The HTTP status is available through the `Code()` accessor.
- `RateLimitedError`: the register's request budget is exhausted (HTTP 429). It carries a `RetryAfter` duration so the caller can requeue with a precise delay.

Match the sentinels with `errors.Is` and `RateLimitedError` with `errors.As`:

```go
for _, check := range report.Checks {
	if check.Status != tin.StatusUnverified {
		continue
	}
	var rl *tin.RateLimitedError
	switch err := check.Err(); {
	case errors.As(err, &rl):
		// Retry after rl.RetryAfter.
	case errors.Is(err, tin.ErrInput):
		// The register rejected the identifier as badly formed.
	case errors.Is(err, tin.ErrServer), errors.Is(err, tin.ErrNetwork):
		// The register did not answer; try again later.
	}
}
```

## Command line

Install the command into your Go environment with:

```bash
go install ./cmd/gobl.tin
```

`verify` takes a GOBL envelope or document that holds an `org.Party` or a `bill.Invoice`, and verifies it with VIES. It prints one line per check, `<path>: <status> (<source>)`, and one indented line per mismatch.

```bash
gobl.tin verify ./test/data/party.json
```

```
/tax_id: valid (vies)
  /name: document "Acme Trading", register "ACME TRADING GMBH"
/identities/0: unsupported
```

Exit codes:

| Code | Meaning |
| --- | --- |
| 0 | Every printed report is valid. |
| 1 | A report is not valid: an `invalid` check, or a party with no identifiers (`no identifiers to verify` on stderr). |
| 2 | A register could not answer: some check is `unverified`. |
| 3 | Usage, file or parse error. |

With `--party both`, code 2 wins over code 1: an `unverified` check in either report exits 2 even when the other report is invalid.

For an invoice, `--party` selects the customer (the default), the supplier, or both. With `both`, each report is headed by its label. On a party document the flag is ignored with a warning on stderr.

```bash
gobl.tin verify --party supplier ./test/data/invoice-valid.json
gobl.tin verify --party both ./test/data/invoice-valid.json
```

`--json` prints the report as JSON. With `--party both` the output is an object keyed by `customer` and `supplier`.

```bash
gobl.tin verify --json ./test/data/party.json
```

## Testing

Tests run offline. Register responses are served by `httptest` servers and the command tests inject a fake verifier, so no test dials the live VIES service. Fuzz tests run their seed corpus under `go test`; run one longer with `go test -fuzz=FuzzNormalizeName -fuzztime=30s .`

```bash
go test -race ./...
```

## Development

[VIES Technical Information](https://ec.europa.eu/taxation_customs/vies/#/technical-information)
