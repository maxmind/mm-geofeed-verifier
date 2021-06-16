## CHANGELOG

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
