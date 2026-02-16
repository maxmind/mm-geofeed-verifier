package verify

import "errors"

var (
	// ErrNotUTF8 indicates a file encoding that is not valid UTF-8 (with
	// optional BOM). RFC 8805 says that "feeds MUST use UTF-8 character
	// encoding". This is a separate error from ErrInvalidGeofeed, because we
	// can't confidently read anything from the file if it's not UTF-8.
	ErrNotUTF8 = errors.New("geofeed is not valid UTF-8")
	// ErrInvalidGeofeed represents error that is returned in case of incomplete
	// compliance with RFC 8805 standards and the mode in which the program is
	// run.
	ErrInvalidGeofeed = errors.New("geofeed does not comply with the RFC 8805 standards")
	// ErrEmptyGeofeed indicates a Geofeed with no records.
	ErrEmptyGeofeed = errors.New("geofeed is empty")
)

// RowInvalidity represents type of row invalidity.
type RowInvalidity int

// Invalidity types.
const (
	FewerFieldsThanExpected RowInvalidity = iota
	EmptyNetwork
	UnableToParseNetwork
	UnableToFindCityRecord
	UnableToFindISPRecord
	InvalidRegionCode
)

// String implements the Stringer interface.
func (ri RowInvalidity) String() string {
	switch ri {
	case FewerFieldsThanExpected:
		return "FewerFieldsThanExpected"
	case EmptyNetwork:
		return "EmptyNetwork"
	case UnableToParseNetwork:
		return "UnableToParseNetwork"
	case UnableToFindCityRecord:
		return "UnableToFindCityRecord"
	case UnableToFindISPRecord:
		return "UnableToFindISPRecord"
	case InvalidRegionCode:
		return "InvalidRegionCode"
	default:
		return "UnknownInvalidityType"
	}
}
