# AEAT

The `aeat` verifier checks Spanish tax IDs (NIF) in the census of AEAT, the Spanish tax agency, through its VNifV2 service. AEAT holds every NIF, not only those registered for intra-EU trade, and gives the registered name. There is no test environment with real taxpayers, so checks run against production.

AEAT requires mutual TLS with a certificate that its electronic office accepts: personal, representative or seal. Put the verifier before VIES:

```go
cert, err := aeat.LoadCertificate(data, password) // PKCS#12 file
if err != nil {
	return err
}
c := tin.New(
	aeat.New(cert, aeat.WithSeal()),
	vies.New(),
)
```

- `aeat.WithSeal()`: sends requests to `www10.agenciatributaria.gob.es`, required for a seal certificate. Without it, requests go to `www1`.
- `aeat.WithTimeout(d)` (default 15 seconds) and `aeat.WithBaseURL(url)`.
- A missing certificate or an invalid base URL is `ErrServer` on the first call.

## Natural persons

A DNI, a NIE, or a K, L or M code belongs to a natural person. AEAT checks it together with the name:

- Without a name, the answer is `ErrInput` ("AEAT needs the name of a natural person to check a DNI or NIE") and no request is sent.
- AEAT accepts small differences in the name, so a valid answer has `name_confirmed` and gives no name mismatch. AEAT writes the name with the surnames first, and the patch writes that form.
- An invalid answer has the reason "AEAT does not identify the DNI or NIE with the name given": the DNI or NIE alone may be registered.
- `PartyKey` returns the name, normalized, so that a cache keys answers by NIF and name.

An entity is identified from the NIF alone.

## Results

| AEAT result | Status | Record |
|---|---|---|
| `Identificado` | `valid` | registered name |
| `Identificado-Baja` | `valid` | registered name, status `inactive` |
| `Identificado-Revocado` | `valid` | registered name, status `revoked` |
| `No identificado -similar` | `invalid` | registered name |
| `No identificado` | `invalid` | none |
| anything else, such as `No procesado` | `ErrServer` | |

- The tax ID is the NIF that AEAT returns: padded to nine characters, and the current NIF of an entity whose first letter changed.
- `B99999997` is a placeholder NIF for operations without an identified party ("OPERACION NO IDENTIFICADA EN CONSTITUCIÓN"). It is `invalid`, without a request.

## Errors

- A SOAP fault with the code `Client` is `ErrInput`; any other fault is `ErrServer`.
- A 403 (a certificate that AEAT does not accept) is `ErrServer` with the code `403`.
- A 429 is a `RateLimitedError` of one minute.

## Command line

`--aeat-cert` takes the PKCS#12 certificate, and `--aeat-seal` marks a seal certificate. The password comes from `AEAT_CERT_PASSWORD` or a `.env` file.

```bash
AEAT_CERT_PASSWORD=... gobl.tin verify --aeat-cert ./cert.p12 ./party.json
```

## Links

- [VNifV2 technical manual](https://sede.agenciatributaria.gob.es/static_files/Sede/Biblioteca/Manual/Tecnicos/WS/030_036_037/Manual_Tecnico_WS_Masivo_Calidad_Datos_Identificativos.pdf), version 1.8
