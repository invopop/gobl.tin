# HMRC

The `hmrc` verifier checks UK VAT numbers in the VAT register of HMRC, the UK tax authority, through its "Check a UK VAT number" API, version 2. It needs the credentials of an application on the [HMRC Developer Hub](https://developer.service.hmrc.gov.uk) subscribed to that API:

```go
c := tin.New(
	hmrc.New(clientID, clientSecret),
	vies.New(),
)
```

- `hmrc.WithSandbox()`: sends requests to `test-api.service.hmrc.gov.uk`, which needs a sandbox application and holds only [HMRC's mock VAT numbers](https://github.com/hmrc/vat-registered-companies-api/tree/main/public/api/conf/2.0/test-data). Without it, requests go to `api.service.hmrc.gov.uk`.
- `hmrc.WithTimeout(d)` (default 15 seconds), `hmrc.WithBaseURL(url)` and `hmrc.WithHTTPClient(c)`.
- Missing credentials or an invalid base URL are `ErrInput` on the first call.

The verifier gets an OAuth token (`client_credentials`, scope `read:vat`), reuses it for its 4 hours, renews it one minute before it expires, and gets a new one once on a 401.

HMRC checks VAT numbers of 9 or 12 digits. The `GD` and `HA` codes of government departments and health authorities are `unsupported`. Northern Ireland (`XI`) VAT numbers are in VIES.

The verifier uses the simple check, which takes only the VAT number, not the requester's own UK VAT number.

## Address

HMRC gives up to five address lines, a postcode and a country code:

- The first line is the `street`.
- When there are more lines, the last one is the `locality` (the post town).
- The lines between them are the `street_extra`, joined with commas.
- The postcode is the `code`, and the country code is the `country`.

Empty lines are skipped. An address with no street and no postcode is no address. The sandbox gives only one line, so the mapping of more lines is not yet checked against real answers.

## Errors

- A 404 with the code `NOT_FOUND` is `invalid`. Any other 404 is `ErrServer`.
- A 401 or 403 from the API, and a 400 or 401 from the token endpoint, are `ErrServer`: they are a configuration problem.

## Command line

`--hmrc-client-id` takes the client ID, and `--hmrc-sandbox` marks a sandbox application. The secret comes from `HMRC_CLIENT_SECRET` or a `.env` file.

```bash
HMRC_CLIENT_SECRET=... gobl.tin verify --hmrc-sandbox --hmrc-client-id ... ./party.json
```

## Links

- [HMRC Check a UK VAT number API](https://developer.service.hmrc.gov.uk/api-documentation/docs/api/service/vat-registered-companies-api/2.0/oas/page), version 2.0
