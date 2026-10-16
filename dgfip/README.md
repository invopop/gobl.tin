# DGFiP

The `dgfip` verifier checks French VAT numbers. It needs no credentials. Put it before VIES, which answers when it fails:

```go
c := tin.New(
	dgfip.New(),
	vies.New(),
)
```

1. **Validity:** the DGFiP dataset [Numéros de TVA intracommunautaire français](https://data.economie.gouv.fr/explore/dataset/numeros-de-tva-intracommunautaires-francais/) holds every active French VAT number. A number in it is `valid`. The dataset is updated on working days only.
2. **Not registered for VAT:** many active businesses have no VAT number, such as a micro-entrepreneur under the franchise en base. Their tax ID is the SIREN, which GOBL normalizes to the VAT form. For a number that the dataset does not hold, but whose key is the one computed from the SIREN, an active company with that SIREN in the Annuaire des Entreprises is `valid` with `Record.Status` `not-vat-registered`. Any other number is `invalid`, without a company query: GOBL skips the checksum of a key with letters, so a made-up key must not pass.
3. **Record:** for a valid number, the [Annuaire des Entreprises](https://annuaire-entreprises.data.gouv.fr/) API gives the Sirene record: the legal name (for a sole trader, the person's name) and the structured address of the head office. A company closed in Sirene has the status `inactive`. A person who opted out of publication in Sirene gets no record. A head office abroad gives no address.
4. **Failures:** any failure, including of the Annuaire des Entreprises, is an error, so that VIES answers. A `429` is a `RateLimitedError`; the dataset allows 50,000 calls a day per IP address and the error waits until the quota resets. The Annuaire des Entreprises allows 7 calls a second.

Options: `dgfip.WithTimeout(d)` and `dgfip.WithHTTPClient(c)`. `dgfip.WithDatasetURL(url)` and `dgfip.WithCompaniesURL(url)` are for tests.

The data is published under the Licence Ouverte 2.0. Source: DGFiP, Numéros de TVA intracommunautaire français; INSEE, Sirene, through the Annuaire des Entreprises.
