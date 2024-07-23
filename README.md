# gobl.tin

Lookup and validate Tax ID Numbers (TIN) included in GOBL documents.

Copyright [Invopop Ltd.](https://invopop.com) 2024. Released publicly under the [Apache License Version 2.0](LICENSE). For commercial licenses please contact the [dev team at invopop](mailto:dev@invopop.com). In order to accept contributions to this library we will require transferring copyrights to Invopop Ltd.

## Usage

### Go Package

Usage of the GOBL TIN lookup library is pretty straight forward. You must first have a GOBL Envelope including an invoice ready to convert. There are some samples here in the test/data directory.

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/gobl.tin"
)

func main() {

	data, _ := os.ReadFile("test/data/invoice-valid.json")

	env := new(gobl.Envelope)
	if err := json.Unmarshal(data, env); err != nil {
		panic(err)
	}

	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		fmt.Errorf("invalid type %T", env.Document)
	}

	ctx := context.Background()
	c := New()

	// You can validate all the taxID in an invoice
	err := c.Lookup(ctx, inv)

	// You can validate each party
	err = c.Lookup(ctx, inv.Customer)
	err = c.Lookup(ctx, inv.Supplier)

	// And you can validate independent Tax IDs
	err = c.Lookup(ctx, inv.Customer.TaxID)
}

```

### Handling errors

There are 4 type of errors when doing lookup:
- ErrInvalid: Error when the lookup service determines invalid the TaxID: could be because of the format or because it wasn't found on the database.
- ErrInput: An error when the input is missing data like the taxId or the party
- ErrNotSupported: An error when the country is not supported by our service
- ErrNetwork: An error from doing the request, could be a 404, 500. This error does not mean that the taxId is wrong but that the request couldn't be made.

```go
err = c.Lookup(ctx, inv)

if err != nil {
	if e, ok := err.(*Error); ok {
		if e.Is(ErrInvalid) {
			// Case where the taxId is invalid
		} else if e.Is(ErrInput) {
			// Case where something from the input is wrong/missing (taxId, party)
		} else if e.Is(ErrNotSupported) {
			// Case where the country code is not supported
		} else if e.Is(ErrNetwork) {
			// Case where there is a error with the network/request
		}
	}
}

```


### Command Line

The GOBL TIN Lookup tool also includes a command line helper. You can install manually in your Go environment with:

```bash
go install ./cmd/gobl.tin
```

You can write the simple command, that will output a message regarding the TIN of the customer:

```bash
gobl.tin lookup ./test/data/invoice-valid.json
```

But you can also define the party you want to validate the TIN as an argument:

```bash
gobl.tin lookup --type customer ./test/data/invoice-valid.json
```

```bash
gobl.tin lookup --type supplier ./test/data/invoice-valid.json
```

```bash
gobl.tin lookup --type both ./test/data/invoice-valid.json
```

## Testing

### testify

The library uses testify for testing. To run the tests you can use the command:
```
go test
```

## Development

[VIES Technical Information](https://ec.europa.eu/taxation_customs/vies/#/technical-information)