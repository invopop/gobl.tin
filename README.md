# gobl.tin

Lookup and validate Tax ID Numbers (TIN) included in GOBL documents.

Copyright [Invopop Ltd.](https://invopop.com) 2024. Released publicly under the [Apache License Version 2.0](LICENSE). For commercial licenses please contact the [dev team at invopop](mailto:dev@invopop.com). In order to accept contributions to this library we will require transferring copyrights to Invopop Ltd.

## Usage

### Go Package

Usage of the GOBL TIN lookup library is pretty straight forward. You must first have a GOBL Envelope including an invoice ready to convert. There are some samples here in the test/data directory.

```go
package gobltin

import (
	"encoding/json"
	"os"

	"github.com/invopop/gobl"
	gobltin "github.com/invopop/gobl.tin"
)

func main() {

	data, _ := os.ReadFile("test/data/invoice-valid.json")

	env := new(gobl.Envelope)
	if err := json.Unmarshal(data, env); err != nil {
		panic(err)
	}

	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return nil, fmt.Errorf("invalid type %T", env.Document)
	}

	valid, err = gobltin.LookupTin(inv.Customer)
	valid, err = gobltin.LookupTin(inv.Supplier)
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