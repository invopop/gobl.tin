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
- **Address:** VIES gives it as one free text, so the record has no address.
- **Status:** VIES has none.
- **Tax ID:** the identity that VIES echoes.

## Errors

VIES reports some failures in the body of an HTTP 200 response, in `errorWrappers` or `userError`:

| VIES code | Error |
|---|---|
| `MS_MAX_CONCURRENT_REQ` and the other concurrency codes | `RateLimitedError`, one minute, with the code as `Reason` |
| `INVALID_INPUT` | `ErrInput` |
| any other, such as `MS_UNAVAILABLE` | `ErrServer` |

## Links

- [VIES Technical Information](https://ec.europa.eu/taxation_customs/vies/#/technical-information)
