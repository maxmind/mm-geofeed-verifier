package verify

import (
	"errors"
	"fmt"
)

// ErrDatabaseLookup indicates a problem with an MMDB itself -- it could not
// be opened, or a lookup against it failed. The geofeed was not evaluated, so
// a consumer must not change the feed's status on this error -- it signals a
// problem with the database, not with the geofeed.
var ErrDatabaseLookup = errors.New("mmdb lookup failed")

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
	// the key of the SampleInvalidRows map this InvalidRow came from, but is
	// included so a value handled independently of that map (for example,
	// passed on its own to a renderer) can still be identified.
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

// String returns engineer-facing text of the form "line 5: <Diagnostic>".
// Like Diagnostic, it must not be shown to a geofeed's owner.
//
// This is a value receiver so that %s formats map[RowInvalidity]InvalidRow
// values too, which are not addressable and so cannot satisfy an interface
// requiring a pointer receiver. It intentionally omits Type -- both of this
// method's call sites already print that themselves -- and it does not
// fall back to Reason when Diagnostic is empty: a bare "line 0: " for a
// zero-valued InvalidRow is correct, and a fallback would make an
// engineer-facing renderer silently emit customer-facing text instead.
func (r InvalidRow) String() string {
	return fmt.Sprintf("line %d: %s", r.Line, r.Diagnostic)
}
