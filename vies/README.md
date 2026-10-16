# VIES

The `vies` verifier checks EU VAT numbers in [VIES](https://ec.europa.eu/taxation_customs/vies/), the European Commission's VAT register. It covers every member state, with `EL` for Greece and `XI` for Northern Ireland. It needs no credentials.

```go
c := tin.New(vies.New(
	vies.WithTimeout(5*time.Second), // default 15 seconds
	vies.WithBaseURL(url),
	vies.WithHTTPClient(httpClient),
))
```

## Record

- **Name:** given by most member states. Some mask it with `---`, which gives no record.
- **Address:** see [Addresses](#addresses).
- **Status:** VIES has none.
- **Tax ID:** the identity that VIES echoes.

## Addresses

VIES gives the address as one free text, written differently by each member state. The verifier has one parser per member state, in `address_<country>.go`, and structures the address only when the whole text has the shape that the parser knows. Any other text gives no address. The register's own form is kept, in upper case.

| Member state | Shape | Address |
|---|---|---|
| `IT` | Two lines: the street with its house number, then the postcode, the locality and the province code. `VIA DEL BOSCO DEGLI ARVALI 84 INT 7` / `00148 ROMA RM` | `street` "VIA DEL BOSCO DEGLI ARVALI", `num` "84", `street_extra` "INT 7", `code` "00148", `locality` "ROMA", `region` "RM" |

The Italian street line must start with a street type, such as `VIA`, `PIAZZA` or `LOC.`, so that a line such as `C/O STUDIO ROSSI` is never written as a street.

A member state without a parser gives no address. To add one, collect real VIES answers for it first, then add its parser and a table of those answers as tests.

## Errors

VIES reports some failures in the body of an HTTP 200 response, in `errorWrappers` or `userError`:

| VIES code | Error |
|---|---|
| `MS_MAX_CONCURRENT_REQ` and the other concurrency codes | `RateLimitedError`, one minute, with the code as `Reason` |
| `INVALID_INPUT` | `ErrInput` |
| any other, such as `MS_UNAVAILABLE` | `ErrServer` |

## Links

- [VIES Technical Information](https://ec.europa.eu/taxation_customs/vies/#/technical-information)
