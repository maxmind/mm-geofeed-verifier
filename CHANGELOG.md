## CHANGELOG

## 3.0.0 (2024-08-14)

* Update interface of ProcessGeofeed in the verifier package, adding a new
  struct to hold verification options. This will make it easier to add/remove
  options in the future.
* Add an option to ProcessGeofeed to reduce the verbosity of error messages,
  toggling whether file paths are included.

## 2.4.0 (2023-07-13)

* Update files to comply with major release version 2
* Automate version fetching from git release tag
* Do not fail immediately on invalid row, but return custom error along with 
  stats and examples of rows that are not compliant with RFC 8805 standard

## 2.3.0 (2023-07-05)

* Compare subdivisions in corrections to most specific, instead of least
  specific, subdivision in MMDB file
* Add optional 'lax' mode that does not require country prefix for ISO-3166 code

## 2.2.0 (2023-03-21)

* Update to Go version 1.18
* Moved ProcessGeofeed to `verify` sub-package to allow the use of this code as a library
* add version argument
* optionally include ISP/ASN information in output

## 2.1.0 (2021-06-16)

* Fix handling of extra fields (reported by Raiko Wielk)
* Compare correction postal code (if it exists) against MMDB postal code
* Only print fields that actually differ between correction and MMDB record; previously
  if any one field had a difference we printed all fields

## 2.0.0 (2021-01-27)

* Can now better handle files with a leading BOM
* Argument names changed for less typing

## 1.0.0 (2020-05-04)

* Initial Release
