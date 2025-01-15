## mm-geofeed-verifier

mm-geofeed-verifier attempts to validate that a given file follows the format
suggested at https://datatracker.ietf.org/doc/html/rfc8805, and
makes some comparisons to a given MMDB, typically the latest available GeoIP2-City.mmdb

## Usage

#### Default strict mode

By default strict mode requires exact ISO-3166-2 format compliance for region codes:

`mm-geofeed-verifier -gf /path/to/geofeed-formatted-file -db /path/to/Database.mmdb`

#### Lax mode

Use `--lax` mode to allow region codes to be provided without ISO-3166 country code prefix:

`mm-geofeed-verifier --lax -gf /path/to/geofeed-formatted-file -db /path/to/Database.mmdb`

## Installation and release

Find a suitable archive for your system on the [Releases
tab](https://github.com/maxmind/mm-geofeed-verifier/releases). Extract the
archive. Inside is the `mm-geofeed-verifier` binary.

## Installation from source or Git

You need the Go compiler (Go 1.23+). You can get it at the [Go
website](https://golang.org).

The easiest way is via `go install`:

    $ go install github.com/maxmind/mm-geofeed-verifier/v3@latest

The program will be installed to `$GOPATH/bin/mm-geofeed-verifier`.

# Bug Reports

Please report bugs by filing an issue with our GitHub issue tracker at
https://github.com/maxmind/mm-geofeed-verifier/issues

# Copyright and License

This software is Copyright (c) 2019 - 2025 by MaxMind, Inc.

This is free software, licensed under the [Apache License, Version
2.0](LICENSE-APACHE) or the [MIT License](LICENSE-MIT), at your option.
