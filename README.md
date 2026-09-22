# gobl.tin

Lookup and validate Tax ID Numbers (TIN) included in GOBL documents.

Copyright [Invopop Ltd.](https://invopop.com) 2024. Released publicly under the [Apache License Version 2.0](LICENSE). For commercial licenses please contact the [dev team at invopop](mailto:dev@invopop.com). In order to accept contributions to this library we will require transferring copyrights to Invopop Ltd.

## Usage

### Go Package

A lookup returns a `Result`, never a bare yes/no error. An invalid TIN is an ordinary `Result` with `Valid` set to `false` and a `nil` error. Errors are reserved for cases where the registry could not answer.

The client holds no cache, so every call reaches the registry. Caching is the consuming application's decision. The registry clients are built once per `Client` and reused, so lookups share connections. Requests time out after 15 seconds by default.

Configure the client with options:

```go
c := tin.New(
	// Pass options through to the VIES client.
	tin.WithVIESOptions(vies.WithTimeout(5*time.Second), vies.WithBaseURL(url)),
	// Or replace the VIES registry entirely, for example with a fake in tests.
	tin.WithVIES(myRegistry),
)
```

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/gobl"
	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/bill"
)

func main() {
	data, _ := os.ReadFile("test/data/invoice-valid.json")

	env := new(gobl.Envelope)
	if err := json.Unmarshal(data, env); err != nil {
		panic(err)
	}

	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		panic(fmt.Errorf("invalid type %T", env.Document))
	}

	ctx := context.Background()
	c := tin.New()

	// Look up the parties of an invoice: "customer", "supplier" or "both".
	res, err := c.LookupInvoice(ctx, inv, tin.InvoicePartyBoth)
	if err != nil {
		panic(err)
	}
	fmt.Println("customer valid:", res.Customer.Valid)
	fmt.Println("supplier valid:", res.Supplier.Valid)

	// Look up a single party.
	pres, err := c.LookupParty(ctx, inv.Customer)
	if err != nil {
		panic(err)
	}
	fmt.Println(pres.Valid, pres.Name, pres.Source)

	// Look up an independent tax identity.
	ires, err := c.LookupIdentity(ctx, inv.Customer.TaxID)
	if err != nil {
		panic(err)
	}
	fmt.Println(ires.Valid)
}
```

A `Result` carries:

- `Valid`: whether the registry recognises the TIN.
- `Name`: the name the registry holds for the party. Empty when the registry masks it, which some member states always do.
- `Address`: the registered address as a single unstructured string. Empty when masked.
- `Source`: the registry that answered, for example `"vies"`.

### Handling errors

An invalid TIN is not an error. The error taxonomy covers only the cases where the registry could not answer:

- `ErrInput`: the input is malformed or incomplete. A missing tax ID, an empty code, or a request the registry rejected as badly formed (HTTP 400).
- `ErrNotSupported`: no registry covers the country.
- `ErrNetwork`: the request failed below HTTP. A dial error, a timeout, or a response that could not be decoded. It says nothing about the TIN.
- `ErrServer`: the registry answered with an unexpected status, for example a 500. The HTTP status is available through the `Code()` accessor.
- `RateLimitedError`: the registry's request budget is exhausted (HTTP 429). It carries a `RetryAfter` duration so the caller can requeue with a precise delay.

Errors that come from an HTTP response carry the status as a string via `Code()`, so callers can distinguish statuses structurally instead of parsing messages. Match the sentinels with `errors.Is` and `RateLimitedError` with `errors.As`:

```go
res, err := c.LookupInvoice(ctx, inv, tin.InvoicePartyBoth)
if err != nil {
	var rl *tin.RateLimitedError
	switch {
	case errors.Is(err, tin.ErrInput):
		// Something in the input is wrong or missing.
	case errors.Is(err, tin.ErrNotSupported):
		// The country code is not supported.
	case errors.As(err, &rl):
		// Retry after rl.RetryAfter.
	case errors.Is(err, tin.ErrServer):
		// The registry failed to answer.
	case errors.Is(err, tin.ErrNetwork):
		// The request could not be made.
	}
	return err
}
if !res.Customer.Valid {
	// The registry does not recognise the customer's TIN.
}
```

### Applying results to a party

`Result.ApplyTo` writes the registry's details back into a GOBL party. It is deliberately conservative: it fills an empty name, keeps a matching name with the user's own casing, and only overwrites a disagreeing name when `ApplyOptions.CorrectName` is set. A disagreement is always reported in `Changes.NameMismatch`. Name comparison folds case, punctuation and surrounding whitespace.

Addresses are never applied: VIES returns the address as a single unstructured string, while GOBL addresses are structured, so applying it would mean guessing at a parse.

```go
res, err := c.LookupParty(ctx, inv.Customer)
if err != nil {
	return err
}
changes, err := res.ApplyTo(inv.Customer, tin.ApplyOptions{})
if err != nil {
	return err
}
if changes.Any() {
	// The party changed and the document needs writing back.
}
if changes.NameMismatch {
	// The party's name disagrees with the registry.
}
```

### Command Line

The GOBL TIN Lookup tool also includes a command line helper. You can install manually in your Go environment with:

```bash
go install ./cmd/gobl.tin
```

The command prints one status line per requested party with the validity, the source registry, and the registered name when the registry disclosed one. It exits with code 0 when every requested TIN is valid, and non-zero on an invalid TIN or an error.

```bash
gobl.tin lookup ./test/data/invoice-valid.json
```

By default the command checks the customer. Select the party with the `--type` flag:

```bash
gobl.tin lookup --type customer ./test/data/invoice-valid.json
gobl.tin lookup --type supplier ./test/data/invoice-valid.json
gobl.tin lookup --type both ./test/data/invoice-valid.json
```

## Testing

Tests run offline. Registry responses are served by `httptest` servers, so no test dials the live VIES service. Run them with:

```bash
go test -race ./...
```

## Development

[VIES Technical Information](https://ec.europa.eu/taxation_customs/vies/#/technical-information)
