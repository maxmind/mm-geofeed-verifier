## CHANGELOG

## 5.0.0 - TBD

- Breaking: `CheckResult.SampleInvalidRows` is now
  `map[RowInvalidity]InvalidRow` rather than `map[RowInvalidity]string`. The
  preformatted sample strings are gone; the same information is available as
  data on `InvalidRow`: `Line`, `Type`, `Fields` (the row's own fields, not a
  rejoined string), a curated and display-safe `Reason`, and internal
  engineer-facing `Diagnostic` text. `InvalidRow.String()` renders
  `line <Line>: <Diagnostic>`, which resembles the old string but reports the
  row's line in the file, counting comment lines, where the old string counted
  only data rows.
- Breaking: `UnableToFindCityRecord` and `UnableToFindISPRecord` are removed. A
  database that can't be opened, or a lookup against it that fails, is a problem
  with the database, not with the geofeed, so `ProcessGeofeed` now returns an
  error wrapping the new `ErrDatabaseLookup` rather than recording the row as
  invalid. Consumers should treat it as operational and leave the geofeed's
  state unchanged.
- Breaking: the module path is now `github.com/maxmind/mm-geofeed-verifier/v5`.
  Update imports accordingly.
- Breaking: `ProcessGeofeed` returns `(Result, error)`. `Result` replaces
  `CheckResult` and absorbs the diff lines and ASN counts that used to be
  separate return values, as `Diffs` and `ASNCounts`. `NewCheckResult` is
  removed; `ProcessGeofeed` is the only thing that builds a `Result`.
- Breaking: a geofeed that fails verification is no longer reported as an error.
  `Result.Failure` names the reason -- `FailureNotUTF8`, `FailureEmpty`,
  `FailureTooManyInvalidRows` or `FailureUnreadableCSV` -- and is `FailureNone`
  when the geofeed passed. `ProcessGeofeed` returns a non-nil error only when
  verification could not be performed at all: the file was unreadable, or an
  MMDB was unusable (`ErrDatabaseLookup`). So `ErrNotUTF8`, `ErrEmptyGeofeed`
  and `ErrInvalidGeofeed` are removed. A file that cannot be parsed as CSV,
  which previously produced an error carrying no sentinel at all and so was
  indistinguishable from an operational fault, is now `FailureUnreadableCSV`.
- Breaking: lines holding only whitespace are skipped, as comment lines already
  are. Previously such a line parsed as a single empty field, counted toward
  `Result.Total`, and was reported as a `FewerFieldsThanExpected` invalid row
  whose `Fields` held one empty string.

## 4.0.0 (2026-02-16)

- Require that geofeeds be encoded as valid UTF-8.

## 3.1.0 (2024-11-05)

- Empty geofeeds will now be considered invalid, unless the (new) EmptyOK option
  is passed to ProcessGeofeed (e.g. via the new `empty-ok` flag).

## 3.0.0 (2024-08-14)

- Update interface of ProcessGeofeed in the verifier package, adding a new
  struct to hold verification options. This will make it easier to add/remove
  options in the future.
- Add an option to ProcessGeofeed to reduce the verbosity of error messages,
  toggling whether file paths are included.

## 2.4.0 (2023-07-13)

- Update files to comply with major release version 2
- Automate version fetching from git release tag
- Do not fail immediately on invalid row, but return custom error along with
  stats and examples of rows that are not compliant with RFC 8805 standard

## 2.3.0 (2023-07-05)

- Compare subdivisions in corrections to most specific, instead of least
  specific, subdivision in MMDB file
- Add optional 'lax' mode that does not require country prefix for ISO-3166 code

## 2.2.0 (2023-03-21)

- Update to Go version 1.18
- Moved ProcessGeofeed to `verify` sub-package to allow the use of this code as
  a library
- add version argument
- optionally include ISP/ASN information in output

## 2.1.0 (2021-06-16)

- Fix handling of extra fields (reported by Raiko Wielk)
- Compare correction postal code (if it exists) against MMDB postal code
- Only print fields that actually differ between correction and MMDB record;
  previously if any one field had a difference we printed all fields

## 2.0.0 (2021-01-27)

- Can now better handle files with a leading BOM
- Argument names changed for less typing

## 1.0.0 (2020-05-04)

- Initial Release
