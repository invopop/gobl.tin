# DGFiP

The `dgfip` verifier checks French VAT numbers. It needs no credentials. Put it before VIES, which answers when it fails:

```go
c := tin.New(
	dgfip.New(),
	vies.New(),
)
```

1. **Validity:** the DGFiP dataset [Numéros de TVA intracommunautaire français](https://data.economie.gouv.fr/explore/dataset/numeros-de-tva-intracommunautaires-francais/) holds every active French VAT number. A number in it is `valid`. The dataset is updated on working days only, so a number that it does not hold is checked live in VIES, and that answer is final.
2. **Record:** for a valid number, the [Annuaire des Entreprises](https://annuaire-entreprises.data.gouv.fr/) API gives the Sirene record: the legal name (for a sole trader, the person's name) and the structured address of the head office. A company closed in Sirene has the status `inactive`. A person who opted out of publication in Sirene gets no record. A head office abroad gives no address.
3. **Failures:** any failure, including of the Annuaire des Entreprises, is an error, so that VIES answers. A `429` is a `RateLimitedError`; the dataset allows 50,000 calls a day per IP address and the error waits until the quota resets. The Annuaire des Entreprises allows 7 calls a second.

Options: `dgfip.WithTimeout(d)`, `dgfip.WithHTTPClient(c)`, and `dgfip.WithLive(v)`, which replaces the VIES live check. `dgfip.WithDatasetURL(url)` and `dgfip.WithCompaniesURL(url)` are for tests.

The data is published under the Licence Ouverte 2.0. Source: DGFiP, Numéros de TVA intracommunautaire français; INSEE, Sirene, through the Annuaire des Entreprises.
