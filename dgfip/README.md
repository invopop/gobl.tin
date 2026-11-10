# DGFiP

The `dgfip` verifier checks French VAT numbers. It needs no credentials. Put it before VIES, so that VIES is never asked for France:

```go
c := tin.New(
	dgfip.New(),
	vies.New(),
)
```

1. **Validity:** the DGFiP dataset [Numéros de TVA intracommunautaire français](https://data.economie.gouv.fr/explore/dataset/numeros-de-tva-intracommunautaires-francais/) holds every active French VAT number. A number in it is `valid`. The dataset is updated on working days only.
2. **Any other number is `invalid`.** The reason points to the `SIREN` identity. The dataset cannot tell a business without a VAT number from a number that does not exist. A business without a VAT number, such as a micro-entrepreneur under the franchise en base, often has its SIREN as tax ID, and GOBL normalizes it to the VAT form. The [`sirene`](../sirene/README.md) verifier checks the SIREN when the party gives it as an identity of type `SIREN`.
3. **No record:** the dataset holds only the number and its dates. A valid check has no name and no address, so no name check and no patch follow. The `sirene` verifier gives the name and the address for a `SIREN` identity.
4. **Failures:** any failure is an error, and the check is `unverified`. A `429` is a `RateLimitedError`; the dataset allows 50,000 calls a day per IP address and the error waits until the quota resets.

Decision (2026-11-04): `dgfip` calls only the dataset. Reason: the Annuaire call only added the name and the address, and a customer who wants them sends the `SIREN` identity. Rejected: the fallback that asked the Annuaire des Entreprises about a number with the key computed from the SIREN, and gave `valid` with `Record.Status` `not-vat-registered`.

Options: `dgfip.WithTimeout(d)` and `dgfip.WithHTTPClient(c)`. `dgfip.WithDatasetURL(url)` is for tests.

The data is published under the Licence Ouverte 2.0. Source: DGFiP, Numéros de TVA intracommunautaire français.
