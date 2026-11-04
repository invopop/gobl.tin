# Sirene

The `sirene` verifier checks French SIREN identities in Sirene, the register of INSEE, through the [Annuaire des Entreprises](https://annuaire-entreprises.data.gouv.fr/) API. It needs no credentials. It covers an entry of `identities` with the type `SIREN` and the country `FR` or no country:

```json
{"name": "Danone", "identities": [{"type": "SIREN", "code": "552032534"}]}
```

```go
c := tin.New(
	dgfip.New(),
	vies.New(),
	sirene.New(),
)
```

Sirene covers only identities, so its place in the list does not matter. A party with a French `tax_id` and a `SIREN` identity gets one check for each.

1. **Validity:** a SIREN that Sirene lists is `valid`. A SIREN that it does not list is `invalid`. A code that is not 9 digits is `ErrInput`, without a request.
2. **Record:** the legal name (for a sole trader, the person's name) and the structured address of the head office. A company closed in Sirene has the status `dissolved`. A person who opted out of publication in Sirene gets no name and no address. A head office abroad gives no address.
3. **Failures:** any failure is an error, and the check is `unverified`. A `429` is a `RateLimitedError`. The Annuaire des Entreprises allows 7 calls a second per IP address.

`sirene.Verifier.Company(ctx, siren)` returns the company for a SIREN, or nil when Sirene does not list it. The `dgfip` verifier uses it for the name and the address of a VAT number.

Options: `sirene.WithTimeout(d)` (default 15 seconds) and `sirene.WithHTTPClient(c)`. `sirene.WithURL(url)` is for tests.

The data is published under the Licence Ouverte 2.0. Source: INSEE, Sirene, through the Annuaire des Entreprises.
