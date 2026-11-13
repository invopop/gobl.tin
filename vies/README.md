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

VIES gives the address as one free text, written differently by each member state. The verifier has one parser per member state, in `address_<country>.go`, and structures the address only when the whole text has the shape that the parser knows. Any other text gives no address. The register's own form is kept: its script and its case.

| Member state | Shape | Address |
|---|---|---|
| `BG` | One line: the street part, then the province, the town or village and the postcode. Each detail of the street part has a label: `№` (number), `бл.` (block), `вх.` (entrance), `ет.` (floor), `ап.` (flat), `офис` (office), `магазин` (shop). `ул. СТАРА ПЛАНИНА №69-71 вх.Б ет.5 ап.27 обл.ХАСКОВО, гр.ХАСКОВО 6300` | `street` "ул. СТАРА ПЛАНИНА", `num` "69-71", `street_extra` "вх.Б ет.5 ап.27", `code` "6300", `locality` "ХАСКОВО", `region` "ХАСКОВО" |
| `IT` | Two lines: the street with its house number, then the postcode, the locality and the province code. `VIA DEL BOSCO DEGLI ARVALI 84 INT 7` / `00148 ROMA RM` | `street` "VIA DEL BOSCO DEGLI ARVALI", `num` "84", `street_extra` "INT 7", `code` "00148", `locality` "ROMA", `region` "RM" |
| `NL` | Two lines: the street with its house number padded to five digits and an optional addition, then the postcode and the town. `ZEKERINGSTRAAT 00039 C` / `1014BV AMSTERDAM` | `street` "ZEKERINGSTRAAT", `num` "39 C", `code` "1014BV", `locality` "AMSTERDAM" |
| `PL` | Two lines: the street with its number and an optional flat, then the postcode and the post town. A first line names the town or village when it differs from the post town. `NIEKANIN` / `SZMARAGDOWA 3 M5` / `78-100 KOŁOBRZEG` | `street` "SZMARAGDOWA", `num` "3", `street_extra` "M5, NIEKANIN", `code` "78-100", `locality` "KOŁOBRZEG" |

The Italian street line must start with a street type, such as `VIA`, `PIAZZA` or `LOC.`, so that a line such as `C/O STUDIO ROSSI` is never written as a street.

The Bulgarian text stays in Cyrillic. The labelled details stay in `street_extra` with their labels, because `block`, `floor` and `door` lose their labels in UBL and CII. A housing estate (`жк`) before a street goes to `street_extra`. The Bulgarian parser gives no address when:

- the postcode is absent. Old records with unlabelled number groups, such as `УЛ.ВЕНЕЦ 028 01 04 007`, have no postcode, and their address can be out of date.
- text without a label follows the details, or the street part has a comma or two words without a space between them.
- a number inside or at the end of the street name hides the house number, as in `бул. ВИТОША 2 В.ТЪРНОВО` or `ул. ОБОРИЩЕ 15 №3`. A bare number at the end of a name without a number sign is the house number: `Максим Горки 22` gives `num` "22". An ordinal (`8-ми`) and the number of a housing estate (`жк МЛАДОСТ 2`) are part of the name.

The Dutch parser drops the padding zeros of the house number. A short addition (`C`, `A-1`, `14`) stays with the number; longer details, such as `UNIT C.2.17`, go to `street_extra`. A shielded address (`ADRES AFGESCHERMD`), a PO box (`POSTBUS`), a foreign address and house number `00000` give no address. The postcode stays as the register writes it, without a space.

The Polish flat (`M` and a digit, as in `M5` or `M2.43`) stays with its label in `street_extra`. A village with a separate post town goes to `street_extra` and the post town to `locality`, as Polish post writes it. Without a post town, the first line is the `locality`. A foreign address, a village without a town, and a street name that hides the house number (`SUCHA 1 LOKAL 5`, `SUCHA 1 MAGAZYN`) give no address.

A member state without a parser gives no address. To add one, collect real VIES answers for it first, then add its parser and a table of those answers as tests.

## Northern Ireland

VIES gives an `XI` address as one line in which the street and the town run together, so it has no parser. An XI number is a UK VAT registration, so `vies.WithNorthernIrelandRecords(v)` takes the record of a valid XI number from `v`, usually the `hmrc` verifier, asked for the GB number with the same digits. VIES still decides validity. When `v` fails or does not hold the number, the VIES record (the name) stays. The command line does this when HMRC credentials are given.

## Errors

VIES reports some failures in the body of an HTTP 200 response, in `errorWrappers` or `userError`:

| VIES code | Error |
|---|---|
| `MS_MAX_CONCURRENT_REQ` and the other concurrency codes | `RateLimitedError`, one minute, with the code as `Reason` |
| `INVALID_INPUT` | `ErrInput` |
| any other, such as `MS_UNAVAILABLE` | `ErrServer` |

## Links

- [VIES Technical Information](https://ec.europa.eu/taxation_customs/vies/#/technical-information)
