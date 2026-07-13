## mm-geofeed-verifier

mm-geofeed-verifier attempts to validate that a given file follows the format
suggested at https://datatracker.ietf.org/doc/html/rfc8805. Optionally, it can
also compare the corrections against a given MMDB, typically the latest
available GeoIP2-City.mmdb, reporting on how many differ from the current
mappings.

## Usage

#### Format validation only

Run with just `-gf` to validate the geofeed's RFC 8805 format. No database is
needed:

`mm-geofeed-verifier -gf /path/to/geofeed-formatted-file`

#### Comparing against an MMDB

Pass `-db` to additionally compare each correction against that MMDB and report
differences:

`mm-geofeed-verifier -gf /path/to/geofeed-formatted-file -db /path/to/Database.mmdb`

#### Default strict mode

By default strict mode requires exact ISO-3166-2 format compliance for region
codes:

`mm-geofeed-verifier -gf /path/to/geofeed-formatted-file`

#### Lax mode

Use `--lax` mode to allow region codes to be provided without ISO-3166 country
code prefix:

`mm-geofeed-verifier --lax -gf /path/to/geofeed-formatted-file`

#### ISP details in comparison output

Pass `-isp <path>` with an ISP MMDB to augment comparison output with AS number,
AS organization, and ISP name for rows that differ. It is only meaningful
together with `-db`; in format-only mode it is ignored (the tool prints a notice
to stderr):

`mm-geofeed-verifier -gf /path/to/geofeed-formatted-file -db /path/to/Database.mmdb -isp /path/to/ISP.mmdb`

## Installation and release

Find a suitable archive for your system on the
[Releases tab](https://github.com/maxmind/mm-geofeed-verifier/releases). Extract
the archive. Inside is the `mm-geofeed-verifier` binary.

## Installation from source or Git

You need the Go compiler (Go 1.25+). You can get it at the
[Go website](https://go.dev).

The easiest way is via `go install`:

    $ go install github.com/maxmind/mm-geofeed-verifier/v4@latest

The program will be installed to `$GOPATH/bin/mm-geofeed-verifier`.

# Bug Reports

Please report bugs by filing an issue with our GitHub issue tracker at
https://github.com/maxmind/mm-geofeed-verifier/issues

# Copyright and License

This software is Copyright (c) 2019 - 2026 by MaxMind, Inc.

This is free software, licensed under the
[Apache License, Version 2.0](LICENSE-APACHE) or the [MIT License](LICENSE-MIT),
at your option.
