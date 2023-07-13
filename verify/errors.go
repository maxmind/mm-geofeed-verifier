package verify

import "errors"

// ErrInvalidGeofeed represents error that is returned in case of incomplete compliance
// with RFC 8805 standards and the mode in which the program is run.
var ErrInvalidGeofeed = errors.New("geofeed does not comply with the RFC 8805 standards")

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
