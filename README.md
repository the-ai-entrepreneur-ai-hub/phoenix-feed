# phoenix-feed

Near real time public safety incident feed for Phoenix metro.

## Status

v0.1 — design phase. No code yet. Architecture, schema, and compliance posture are locked.

## Documents

- [`docs/architecture.md`](docs/architecture.md) — system design, components, data model, polling logic, API surface, deployment.
- [`docs/compliance-memo.md`](docs/compliance-memo.md) — legal posture, why we operate under the Phoenix Open Data Policy and not under ARS § 39-121.03, what changes for the paid tier.
- [`docs/commercial-purpose-letter-template.md`](docs/commercial-purpose-letter-template.md) — reusable § 39-121.03(A) statement template, kept on file for any future agency that requires formal records request handling.
- [`db/schema.sql`](db/schema.sql) — Postgres + PostGIS DDL.

## Build the PDFs

```
python pdf-build/build.py                # compliance memo
python pdf-build/build_template.py       # commercial purpose letter
python pdf-build/build_architecture.py   # architecture doc (TODO)
```

PDFs land in `pdf-build/output/`.

## Toolchain

- **Backend**: Go 1.22+
- **Database**: Postgres 16 with PostGIS 3.4
- **Build**: Pandoc + Graphviz + Chrome headless for PDFs
- **Hosting (phase 1)**: Dan's box via ngrok SSH tunnel
- **Hosting (phase 2)**: TBD, likely DO droplet with managed Postgres

## Compliance summary

- Polls Phoenix Fire's ArcGIS REST endpoint at 60s + jitter under the Phoenix Open Data Policy.
- Users hit our cache only; never the city.
- Visible attribution; no Phoenix logos or seals; persistent "not for emergency use" banner.
- Aggregate liability cap and "as is" warranty in EULA.
- Scanner + Whisper as the secondary source (Mesa, Tucson, Phoenix fallback). Tagged separately, not treated as equivalent.

See `docs/compliance-memo.md` for the full reasoning.

## Repo layout

See `docs/architecture.md` section 11.

## License

TBD.
