# gobl.tin

Verify the identifiers of GOBL parties against the registers that issue them, read what the register holds, and patch the party.

Copyright [Invopop Ltd.](https://invopop.com) 2026. Released publicly under the [Apache License Version 2.0](LICENSE). For commercial licenses please contact the [dev team at invopop](mailto:dev@invopop.com). In order to accept contributions to this library we will require transferring copyrights to Invopop Ltd.

## Vocabulary

- A **party** is a GOBL `org.Party`.
- An **identifier** is the party's `tax_id`. Entries of `identities` are not checked.
- A **verifier** is one register client. `vies.New()` and `aeat.New()` are the built-in verifiers.
- A **register** is the external authority a verifier asks, for example VIES or the AEAT census.
- A **check** is the outcome for one identifier.
- A **report** is the `Report` for one party: one check per identifier.
- A **record** is what the register holds for an identifier.
- A **mismatch** is one field where the party disagrees with the record.
- A **patch** writes what the register holds back into the party.

## Go package

### Verify a party

`tin.New` takes an ordered list of verifiers. For the `tax_id` of a party, the first verifier that supports it answers. `Client.Verify` returns an error only for a nil party. Everything the registers say, including a register that cannot answer, is in the report.

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
		Name:  "Google Ireland",
		TaxID: &tax.Identity{Country: "IE", Code: "6388047V"},
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

A single tax ID has its own method. It returns a check with an empty `Path`, because there is no party document to point into; its mismatch paths are relative to the tax ID, `/code` and `/country`. A tax ID that no verifier covers is a check with status `unsupported`, not an error:

```go
check, err := c.VerifyTaxID(ctx, &tax.Identity{Country: "DE", Code: "282741168"})
```

Coverage is queryable without a request, over the client's verifiers:

```go
c.SupportsTaxID(&tax.Identity{Country: "ES", Code: "B85905495"}) // true with VIES
c.SupportsTaxID(&tax.Identity{Country: "US", Code: "123456789"}) // false with VIES
```

### Configure VIES

```go
c := tin.New(vies.New(
	vies.WithTimeout(5*time.Second),
	vies.WithBaseURL(url),
	vies.WithHTTPClient(httpClient),
))
```

### Configure AEAT

The `aeat` verifier asks the census of AEAT, the Spanish tax agency, through its VNifV2 identification service. AEAT requires mutual TLS, so `aeat.New` takes the client certificate. Use a certificate that the AEAT electronic office accepts: a personal, representative or seal certificate. Put it before VIES, so that it answers for Spanish tax IDs:

```go
c := tin.New(
	aeat.New(cert, aeat.WithSeal()), // cert is a tls.Certificate
	vies.New(),
)
```

- `aeat.WithSeal()` sends requests to `www10.agenciatributaria.gob.es`, which AEAT requires for a seal certificate. Without it, requests go to `www1`.
- `aeat.WithTimeout(d)` and `aeat.WithBaseURL(url)` work as they do for VIES. The default timeout is 15 seconds.
- A missing certificate or an invalid base URL is `ErrInput` on the first call.

AEAT has no test environment with real taxpayers, so checks always run against production, as they do for VIES.

AEAT needs the name of a natural person, so the verifier sends the party's name with the NIF. A NIF of a natural person (a DNI, a NIE, or a K, L or M code) without a name is `ErrInput`, and no request is sent. This includes `VerifyTaxID`, which has no party. An entity is identified from the NIF alone.

### Write a verifier

A verifier implements `tin.Verifier`:

```go
type Verifier interface {
	Source() cbc.Key
	Supports(id Identifier) bool
	Verify(ctx context.Context, req Request) (*Answer, error)
}
```

`Identifier` describes the identifier: its path in the party, its country and its normalized code. `Supports` routes on it alone. `Request` is what `Verify` receives: the `Identifier` and the `Party` it belongs to, as read-only context for registers that need more than the code, such as a name. `Party` is nil for `VerifyTaxID`; a verifier that needs it returns `ErrInput`. Consumers never construct either; the client builds them from GOBL types.

`Answer` is only what the register knows: `Status` (`valid` or `invalid`), the `Record` when disclosed, the `TaxID` the register echoes, and `CheckedAt` (zero means now). The client builds the check around it: path, identifier, source, mismatches. A verifier returns an error only when the register cannot answer. An invalid identifier is an answer with status `invalid` and a nil error. Any status other than `valid` or `invalid`, including empty, makes the check `unverified`.

To use a new verifier in the command line, add a `register` in its own file under `cmd/gobl.tin`, as `aeat.go` does, and list it in `defaultRegisters` in routing order. A register adds its own flags and builds its verifier, so the `verify` command needs no change.

## Report

A report has one check per identifier: the `tax_id`. A tax ID whose code is empty after normalization gets no check, and the report is then empty. A check carries:

- `path`: the JSON Pointer ([RFC 6901](https://www.rfc-editor.org/rfc/rfc6901)) of the identifier in the party, `/tax_id`.
- `tax_id`: the normalized tax ID. VIES and AEAT set it to the identity the register echoes. AEAT pads the NIF to nine characters, and gives the current NIF of an entity whose first letter changed with its legal form.
- `status`: `valid`, `invalid`, `unsupported` or `unverified`.
- `source`: the verifier that answered, for example `vies`. Empty when unsupported.
- `checked_at`: when the register answered. Absent when unsupported or unverified.
- `record`: what the register holds, when disclosed: the `name`, and the `address` only when the register gives it structured. VIES discloses the name for most member states; some mask it with `---`, which gives no name. VIES gives the address as one free text, which is never parsed, so a VIES record has no address. AEAT gives the name and no address.
- `mismatches`: fields where the party disagrees with the record. Each carries a typed `field` (`name` or `tax_id`), the JSON Pointer `path` of the value (`/name`, `/tax_id/code`, `/tax_id/country`), the `document` value and the `register` value. The client compares the name with `NameMatches`, which folds case, punctuation and whitespace.
- `failure`: the register failure message, only when the status is `unverified`. The Go error is available through `Check.Err()`.

Every path is a JSON Pointer relative to the party document. A consumer that patches an invoice prefixes the party's location, `/customer` or `/supplier`. Go consumers match on the typed values, `Check.TaxID` and `Mismatch.Field`, rather than parsing paths; the pointer is for JSON consumers.

```json
{"checks":[{"path":"/tax_id","tax_id":{"country":"IE","code":"6388047V"},"status":"valid","source":"vies","checked_at":"2026-09-25T09:06:48Z","record":{"name":"GOOGLE IRELAND LIMITED"},"mismatches":[{"field":"name","path":"/name","document":"Google Ireland","register":"GOOGLE IRELAND LIMITED"}]}]}
```

`Report.Valid()` is true when every check that a verifier answered is `valid`. A report with an `invalid` or `unverified` check is not valid. A report where every check is `unsupported` is not valid either: no identifier is verified.

### Record status

`Record.Status` is the standing of the party in its register: `active`, `inactive` or `dissolved`. Verifiers map register values onto this vocabulary and leave it empty for values they do not know. It does not change the check status: a recognised but dissolved company is `valid` with `Record.Status` `dissolved`, and the consumer decides. VIES has no standing and leaves it empty.

AEAT gives one result per NIF. The `aeat` verifier maps it as follows:

| AEAT result | Check status | Record |
|---|---|---|
| `Identificado` | `valid` | The registered name. For a natural person, AEAT gives the name only when the name sent is equal or very similar. |
| `Identificado-Baja` | `valid` | The registered name, with status `inactive`. |
| `Identificado-Revocado` | `valid` | The registered name, with status `dissolved`. AEAT revoked the NIF. |
| `No identificado -similar` | `invalid` | The registered name of the natural person, so the name mismatch shows it. An invalid check is never patched. |
| `No identificado` | `invalid` | None. AEAT echoes the name sent, which is not a record. |

Any other result, such as `No procesado`, is `ErrServer`.

## Patch

`tin.Patch(party, report)` builds an [RFC 7396](https://www.rfc-editor.org/rfc/rfc7396) JSON merge patch on the party, with what the register holds and the party does not. It is a best effort for each field the register gives in a form GOBL can take as is. A patch with nothing to write is `{}`. A nil party or report is `ErrInput`.

```go
patch, err := tin.Patch(party, report)
```

| Field | Written when | Effect |
|---|---|---|
| `name` | The register gives a name that is not exactly the party's | The register's name replaces the party's. A masked name (`---`) is no name, so it is never written. The comparison is exact, so a name that differs only by case is written although the report has no name mismatch, which folds case. |
| `addresses` | The register gives a structured address that is not already the party's first | The registered address becomes the first address. When the party holds it further down, it moves to the front and nothing is dropped; otherwise it replaces the first address and the others stay. Addresses are compared by the fields that locate them, so a label alone is no address, and the same place under another label is not written again. A free-text address, as VIES gives, is never parsed or written. |
| `tax_id` | Never | It is what was verified. |

Checks are read in report order, and the first valid record that has a value for a field wins. Records behind `invalid`, `unverified` or `unsupported` checks are never read. A merge patch replaces arrays whole, so `addresses` always appears as the full array. Applied once, a patch leaves nothing to write: a second patch from the same register is `{}`. With VIES or AEAT, a patch writes at most the name.

## Errors

An invalid identifier is a status, not an error. A register that cannot answer is a check with status `unverified` that carries the failure; `Verify` still returns a nil error. `Check.Err()` returns the Go error so that callers can match it:

- `ErrInput`: the input is malformed or incomplete. A nil party, an identifier without a code, or a request the register rejected as badly formed (HTTP 400).
- `ErrNetwork`: the request failed below HTTP. A dial error, a timeout, or a response that could not be decoded. It says nothing about the identifier.
- `ErrServer`: the register answered with an unexpected status, for example a 500. The HTTP status is available through the `Code()` accessor.
- `RateLimitedError`: the register's request budget is exhausted (HTTP 429). It carries a `RetryAfter` duration so the caller can requeue with a precise delay, and a `Reason` when the register names the limit.

VIES also reports failures in the body of an HTTP 200 response: with `actionSucceed` false and an error code in `errorWrappers`, or with `valid` false and the code in `userError`. The `vies` verifier maps them by meaning: `MS_MAX_CONCURRENT_REQ` and the other concurrency codes are a `RateLimitedError` with a one-minute `RetryAfter` and the code as `Reason`; `INVALID_INPUT` is `ErrInput`; every other code, such as `MS_UNAVAILABLE`, is `ErrServer`. The VIES code is part of the error message.

AEAT reports a badly formed request with a SOAP fault. The `aeat` verifier maps a fault with the code `Client` to `ErrInput` and any other fault to `ErrServer`, with the fault text as the message. AEAT answers 403 to a certificate that it does not accept, which is `ErrServer` with the code `403`.

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

`verify` takes a GOBL envelope or document that holds an `org.Party` or a `bill.Invoice`, and verifies it with VIES, or with AEAT for a Spanish tax ID when you give an AEAT certificate. It prints one line per check, `<path>: <status> (<source>)`, and one indented line per mismatch.

```bash
gobl.tin verify ./test/data/party.json
```

```
/tax_id: valid (vies)
```

A member state that masks trader details gives the check no record and no name comparison; that is what this example shows. When the register discloses the name, a difference prints as an indented mismatch line:

```
/tax_id: valid (vies)
  /name: document "Google Ireland", register "GOOGLE IRELAND LIMITED"
```

Exit codes:

| Code | Meaning |
| --- | --- |
| 0 | Every printed report is valid. |
| 1 | A report is not valid: an `invalid` check, or a party with no identifiers (`no identifiers to verify` on stderr). |
| 2 | Some check is `unverified`: the register could not answer, or rejected the identifier as badly formed (HTTP 400, or an AEAT client fault), or a natural person has no name for AEAT. Exit code 3 covers the command's own input, not the document's. |
| 3 | Usage, file or parse error, including an AEAT certificate that cannot be read. |

With `--party both`, code 2 wins over code 1: an `unverified` check in either report exits 2 even when the other report is invalid.

For an invoice, `--party` selects the customer (the default), the supplier, or both. With `both`, each report is headed by its label. On a party document the flag is ignored with a warning on stderr.

```bash
gobl.tin verify --party supplier ./test/data/invoice-valid.json
gobl.tin verify --party both ./test/data/invoice-valid.json
```

`--aeat-cert` takes a PKCS#12 certificate for AEAT. The command reads its password from the `AEAT_CERT_PASSWORD` environment variable or from a `.env` file, so that the password never appears in the process list. Add `--aeat-seal` for a seal certificate.

```bash
AEAT_CERT_PASSWORD=... gobl.tin verify --aeat-cert ./cert.p12 ./party.json
```

`--json` prints the report as JSON. With `--party both` the output is an object keyed by `customer` and `supplier`.

```bash
gobl.tin verify --json ./test/data/party.json
```

## Testing

Tests run offline. Register responses are served by `httptest` servers and the command tests inject a fake verifier, so no test dials the live VIES or AEAT services. Fuzz tests run their seed corpus under `go test`; run one longer with `go test -fuzz=FuzzNormalizeName -fuzztime=30s .`

```bash
go test -race ./...
```

## Development

- [VIES Technical Information](https://ec.europa.eu/taxation_customs/vies/#/technical-information)
- [AEAT VNifV2 technical manual](https://sede.agenciatributaria.gob.es/static_files/Sede/Biblioteca/Manual/Tecnicos/WS/030_036_037/Manual_Tecnico_WS_Masivo_Calidad_Datos_Identificativos.pdf), version 1.8
