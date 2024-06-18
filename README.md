# gobl.tin
Lookup and validate Tax ID Numbers (TIN) included in GOBL documents.

Copyright [Invopop Ltd.](https://invopop.com) 2024. Released publicly under the [Apache License Version 2.0](LICENSE). For commercial licenses please contact the [dev team at invopop](mailto:dev@invopop.com). In order to accept contributions to this library we will require transferring copyrights to Invopop Ltd.

## Usage

### Go Package

Usage of the GOBL TIN lookup library is pretty straight forward. You must first have a GOBL Envelope including an invoice ready to convert. There are some samples here in the test/data directory.

```go
package main

package main

import (
	"encoding/json"
	"fmt"
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

	tin, err := gobltin.NewTinNumber(env)
	if err != nil {
		panic(err)
	}

	response, err := tin.Lookup()
	if err != nil {
		panic(err)
	}

	//With the output you can check the following fields:
	//response.CountryCode: "DE"
	//response.TinNumber: "123456789"
	//response.RequestDate: "2021-09-29T15:00:00Z"
	//response.Valid: true or false

	fmt.Printf("Country Code: %s\n", response.CountryCode)
	fmt.Printf("VAT Number: %s\n", response.TinNumber)
	fmt.Printf("Request Date: %s\n", response.RequestDate)
	fmt.Printf("Valid: %t\n", response.Valid)
}
```

## Testing

### testify

The library uses testify for testing. To run the tests you can use the command:
```
go test
```

## Development

[VIES Technical Information](https://ec.europa.eu/taxation_customs/vies/#/technical-information)