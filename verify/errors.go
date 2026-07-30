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
	// UnknownInvalidity is the zero value of RowInvalidity. It is not a real
	// invalidity: it is what a RowInvalidity reads as when unset, so that a
	// zero-valued InvalidRow does not silently claim to be
	// FewerFieldsThanExpected.
	UnknownInvalidity RowInvalidity = iota
	FewerFieldsThanExpected
	EmptyNetwork
	UnableToParseNetwork
	UnableToFindCityRecord
	UnableToFindISPRecord
	InvalidRegionCode
)

// String implements the Stringer interface.
func (ri RowInvalidity) String() string {
	switch ri {
	case UnknownInvalidity:
		return "UnknownInvalidity"
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

// InvalidRow describes a sample row that failed verification.
type InvalidRow struct {
	// Line is the 1-based line number of the row in the geofeed file,
	// counting comment and blank lines.
	Line int
	// Type is the kind of invalidity found in the row. It is redundant with
	// the key of the SampleInvalidRowDetails map this InvalidRow came from,
	// but is included so a value handled independently of that map (for
	// example, passed on its own to a renderer) can still be identified.
	// The zero value is UnknownInvalidity, not FewerFieldsThanExpected.
	Type RowInvalidity
	// Fields holds the row's fields as parsed from the file, before any
	// trimming performed during verification. Only the fields verification
	// examines are included: all of them for a too-short row, otherwise the
	// first five. Fields is not a CSV encoding: joining it with commas is
	// not guaranteed to reproduce the original row, since a field itself
	// containing a comma or a quote will re-split differently.
	Fields []string
	// Reason is curated, human-readable text describing why the row is
	// invalid, suitable for display to the geofeed's owner. It is one of a
	// small set of fixed messages per invalidity type: it never
	// interpolates the row's fields (see Fields) or any value derived from
	// them, such as a network address after parsing, and never contains
	// internal error text or library names. Its wording may change at any
	// time: treat it as unstable diagnostic text, not something to parse.
	Reason string
	// Diagnostic is internal, engineer-facing text describing the failure.
	// It is unstable, must not be parsed, and must not be shown to a
	// geofeed's owner -- use Reason for that.
	Diagnostic string
}
