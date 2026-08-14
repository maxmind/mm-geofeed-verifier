package verify

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type processGeofeedTest struct {
	gf      string
	db      string
	dl      []string
	c       Result
	laxMode bool
	emptyOK bool
}

// withoutDiffs returns res with Diffs cleared. Table cases pin the counts
// and the sample invalid rows exactly, but not the diff text: spelling out
// the full difference explanations is tedious and brittle, so the cases that
// care about them assert substrings separately.
func withoutDiffs(res Result) Result {
	res.Diffs = nil
	return res
}

func TestProcessGeofeed_Valid(t *testing.T) {
	goodTests := []processGeofeedTest{
		{
			gf: "testdata/geofeed-valid.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: Result{
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
				ASNCounts:         map[uint]int{},
			},
			laxMode: false,
		},
		{
			gf: "testdata/geofeed-valid.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: Result{
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
				ASNCounts:         map[uint]int{},
			},
			laxMode: true,
		},
		{
			gf: "testdata/geofeed-valid-lax.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: Result{
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
				ASNCounts:         map[uint]int{},
			},
			laxMode: true,
		},
		{
			gf: "testdata/geofeed-valid-optional-fields.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: Result{
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
				ASNCounts:         map[uint]int{},
			},
			laxMode: false,
		},
		{
			gf: "testdata/geofeed-valid-utf8-bom.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: Result{
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
				ASNCounts:         map[uint]int{},
			},
			laxMode: false,
		},
		{
			gf: "testdata/empty.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: Result{
				Total:             0,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
				ASNCounts:         map[uint]int{},
			},
			emptyOK: true,
		},
	}

	// Testing the full content of the difference explanation strings is likely to be
	// tedious and brittle, so we will just check for some substrings.
	for _, test := range goodTests {
		t.Run(
			test.gf+" "+test.db, func(t *testing.T) {
				res, err := ProcessGeofeed(
					test.gf,
					test.db,
					"",
					Options{
						EmptyOK: test.emptyOK,
						LaxMode: test.laxMode,
					},
				)
				require.NoError(t, err, "processGeofeed ran without error")
				for i, s := range test.dl {
					assert.Contains(
						t,
						res.Diffs[i],
						s,
						"got expected substring: '%s', substring",
					)
				}
				assert.Equal(
					t,
					test.c,
					withoutDiffs(res),
					"processGeofeed returned expected results",
				)
			},
		)
	}
}

func TestProcessGeofeed_Invalid(t *testing.T) {
	badTests := []processGeofeedTest{
		{
			gf: "testdata/geofeed-invalid-missing-fields.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: Result{
				Total:       2,
				Differences: 0,
				Invalid:     2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{
					FewerFieldsThanExpected: {
						Line:   1,
						Type:   FewerFieldsThanExpected,
						Fields: []string{"2a02:ecc0::/29", "US", "US-NJ", "Parsippany"},
						Reason: "The row has 4 fields, but a geofeed row requires 5.",
						Diagnostic: "expected 5 fields but got 4, " +
							"row: '2a02:ecc0::/29,US,US-NJ,Parsippany'",
					},
				},
				ASNCounts: map[uint]int{},
				Failure:   FailureInvalidRows,
			},
			laxMode: false,
		},
		{
			gf: "testdata/geofeed-invalid-empty-network.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: Result{
				Total:       2,
				Differences: 1,
				Invalid:     1,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{
					EmptyNetwork: {
						Line:       2,
						Type:       EmptyNetwork,
						Fields:     []string{"", "", "", "", ""},
						Reason:     "The network field is empty.",
						Diagnostic: "network field is empty, row: ',,,,'",
					},
				},
				ASNCounts: map[uint]int{},
				Failure:   FailureInvalidRows,
			},
			laxMode: false,
		},
		{
			gf: "testdata/geofeed-invalid-network.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: Result{
				Total:       2,
				Differences: 1,
				Invalid:     1,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{
					UnableToParseNetwork: {
						Line:   1,
						Type:   UnableToParseNetwork,
						Fields: []string{"2a02:/29", "", "", "", ""},
						// Reason is curated, unlike Diagnostic: it doesn't
						// repeat the netip.ParsePrefix error text or the
						// network field's value, so it isn't coupled to a
						// specific Go version's exact wording of that error,
						// and never quotes a value that parsing may have
						// altered (e.g. a synthesized /32 or /64).
						Reason: "The network field is not a valid IP address or CIDR.",
						Diagnostic: `unable to parse network 2a02:/29: netip.ParsePrefix("2a02:/29"): ` +
							`ParseAddr("2a02:"): colon must be followed by more characters (at ":")`,
					},
				},
				ASNCounts: map[uint]int{},
				Failure:   FailureInvalidRows,
			},
			laxMode: false,
		},
		{
			// Geofeed that is valid in lax mode should not be valid if laxMode == true.
			gf: "testdata/geofeed-valid-lax.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: Result{
				Total:       3,
				Differences: 1,
				Invalid:     2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{
					// Fields keeps the trailing whitespace inside the country
					// and city fields: it is a snapshot from before
					// verifyCorrection trims its (aliased) copy, unlike the
					// curated Reason and Diagnostic text.
					InvalidRegionCode: {
						Line:   1,
						Type:   InvalidRegionCode,
						Fields: []string{"2a02:ecc0::/29 ", "US", "NJ ", "Parsippany ", ""},
						Reason: "The region code is not in ISO 3166-2 format (for example, US-CA). " +
							"Enable lax mode to accept a region code without the country prefix.",
						Diagnostic: "invalid ISO 3166-2 region code format in strict (default) mode, " +
							"row: '2a02:ecc0::/29,US,NJ,Parsippany,'",
					},
				},
				ASNCounts: map[uint]int{},
				Failure:   FailureInvalidRows,
			},
			laxMode: false,
		},
		{
			gf: "testdata/empty.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: Result{
				Total:             0,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
				ASNCounts:         map[uint]int{},
				Failure:           FailureEmpty,
			},
			emptyOK: false,
		},
	}

	for _, test := range badTests {
		t.Run(
			test.gf+" "+test.db, func(t *testing.T) {
				res, err := ProcessGeofeed(
					test.gf,
					test.db,
					"",
					Options{
						EmptyOK: test.emptyOK,
						LaxMode: test.laxMode,
					},
				)
				// A geofeed that fails verification is not an error: the
				// verdict is res.Failure, which test.c pins exactly.
				require.NoError(t, err)
				assert.Equal(t, test.c, withoutDiffs(res))
			},
		)
	}
}

func TestProcessGeofeed_FormatOnly(t *testing.T) {
	t.Run("valid feed, format-only", func(t *testing.T) {
		res, err := ProcessGeofeed(
			"testdata/geofeed-valid.csv",
			"",
			"",
			Options{},
		)
		require.NoError(t, err, "processGeofeed ran without error")
		assert.Equal(t, FailureNone, res.Failure, "expected no verification failure")
		assert.Equal(t, 3, res.Total, "expected total rows")
		assert.Equal(t, 0, res.Differences, "expected no differences")
		assert.Equal(t, 0, res.Invalid, "expected no invalid rows")
		assert.Empty(t, res.Diffs, "expected no diff lines")
		assert.Empty(t, res.ASNCounts, "expected no asn counts")
	})

	t.Run("malformed feed, format-only", func(t *testing.T) {
		res, err := ProcessGeofeed(
			"testdata/geofeed-invalid-missing-fields.csv",
			"",
			"",
			Options{},
		)
		require.NoError(t, err)
		assert.Equal(t, FailureInvalidRows, res.Failure)
		assert.Contains(
			t,
			res.SampleInvalidRows,
			FewerFieldsThanExpected,
			"expected a FewerFieldsThanExpected entry",
		)
	})

	t.Run("bad region code in strict mode, format-only", func(t *testing.T) {
		res, err := ProcessGeofeed(
			"testdata/geofeed-valid-lax.csv",
			"",
			"",
			Options{LaxMode: false},
		)
		require.NoError(t, err)
		assert.Equal(t, FailureInvalidRows, res.Failure)
		assert.Contains(
			t,
			res.SampleInvalidRows,
			InvalidRegionCode,
			"expected an InvalidRegionCode entry",
		)
		assert.Equal(t, 2, res.Invalid, "expected two invalid rows")
		assert.Equal(t, 0, res.Differences, "expected no differences")
	})

	t.Run("lax mode accepts non-prefixed region codes, format-only", func(t *testing.T) {
		res, err := ProcessGeofeed(
			"testdata/geofeed-valid-lax.csv",
			"",
			"",
			Options{LaxMode: true},
		)
		require.NoError(t, err, "processGeofeed ran without error in lax format-only mode")
		assert.Equal(t, FailureNone, res.Failure, "expected no verification failure")
		assert.Equal(t, 3, res.Total, "expected total rows")
		assert.Equal(t, 0, res.Invalid, "expected no invalid rows in lax mode")
		assert.Equal(t, 0, res.Differences, "expected no differences")
		assert.Empty(t, res.Diffs, "expected no diff lines")
		assert.Empty(t, res.ASNCounts, "expected no asn counts")
	})

	t.Run("empty mmdb path opens no DB", func(t *testing.T) {
		_, err := ProcessGeofeed(
			"testdata/geofeed-valid.csv",
			"",
			"",
			Options{},
		)
		require.NoError(t, err, "processGeofeed ran without error when mmdbFilename is empty")
	})

	t.Run("missing mmdb path still errors", func(t *testing.T) {
		_, err := ProcessGeofeed(
			"testdata/geofeed-valid.csv",
			"testdata/does-not-exist.mmdb",
			"",
			Options{},
		)
		require.Error(t, err, "processGeofeed errors when a non-empty mmdbFilename does not exist")
	})
}

// TestInvalidRowZeroValueTypeIsUnknown guards against RowInvalidity's zero
// value silently meaning FewerFieldsThanExpected. Without the
// UnknownInvalidity sentinel occupying iota 0, this would fail.
func TestInvalidRowZeroValueTypeIsUnknown(t *testing.T) {
	var zero InvalidRow
	assert.Equal(t, UnknownInvalidity, zero.Type)
	assert.NotEqual(t, FewerFieldsThanExpected, zero.Type)
}

func TestSampleInvalidRowReportsFileLine(t *testing.T) {
	res, err := ProcessGeofeed(
		"testdata/comments-then-short-row.csv",
		"testdata/GeoIP2-City-Test.mmdb",
		"",
		Options{},
	)
	require.NoError(t, err)
	require.Equal(t, FailureInvalidRows, res.Failure)

	detail, ok := res.SampleInvalidRows[FewerFieldsThanExpected]
	require.True(t, ok, "expected a sample for FewerFieldsThanExpected")

	// The short row is on file line 5: Line is the true file line, counting
	// the three comment lines above it (lines 1, 2, and 4), not a count of
	// data rows (which would be 2: lines 3 and 5 are the only data rows up
	// to and including the short one).
	assert.Equal(t, 5, detail.Line)
	assert.Equal(t, FewerFieldsThanExpected, detail.Type)
	assert.Equal(t, []string{"2.0.0.0/24", "NL", "NL-NH", "Amsterdam"}, detail.Fields)
	assert.Equal(
		t,
		"expected 5 fields but got 4, row: '2.0.0.0/24,NL,NL-NH,Amsterdam'",
		detail.Diagnostic,
	)
}

// TestSampleInvalidRowReportsFileLineForVerifyCorrection covers the second
// SampleInvalidRows population site: rows that reach verifyCorrection (as
// opposed to the too-few-fields site covered by
// TestSampleInvalidRowReportsFileLine above). Reverting this site's Line
// back to the comment-skipping row counter must fail this test.
func TestSampleInvalidRowReportsFileLineForVerifyCorrection(t *testing.T) {
	res, err := ProcessGeofeed(
		"testdata/comments-then-empty-network.csv",
		"testdata/GeoIP2-City-Test.mmdb",
		"",
		Options{},
	)
	require.NoError(t, err)
	require.Equal(t, FailureInvalidRows, res.Failure)

	detail, ok := res.SampleInvalidRows[EmptyNetwork]
	require.True(t, ok, "expected a sample for EmptyNetwork")

	// The empty-network row is on file line 5: Line counts the three
	// comment lines above it (lines 1, 2, and 4); Total, by contrast,
	// counts only data rows (lines 3 and 5), so it is 2 here.
	assert.Equal(t, 5, detail.Line)
	assert.Equal(t, EmptyNetwork, detail.Type)
	assert.Equal(t, []string{"", "", "", "", ""}, detail.Fields)
	assert.Equal(t, 2, res.Total)
	assert.Equal(t, "network field is empty, row: ',,,,'", detail.Diagnostic)
}

// TestProcessGeofeedSkipsWhitespaceOnlyLines proves a line holding only
// spaces or tabs is skipped the way a comment line is, rather than counted as
// a row and reported as one with too few fields. It also pins the physical
// line of the row that genuinely is invalid: skipping records must not shift
// the lines reported for the rows that are kept.
func TestProcessGeofeedSkipsWhitespaceOnlyLines(t *testing.T) {
	res, err := ProcessGeofeed(
		"testdata/whitespace-then-short-row.csv",
		"testdata/GeoIP2-City-Test.mmdb",
		"",
		Options{},
	)
	require.NoError(t, err)
	assert.Equal(t, FailureInvalidRows, res.Failure)
	// The whitespace on file lines 3 and 4 is not two more rows: only the
	// two data rows count, and only the short one is invalid.
	assert.Equal(t, 2, res.Total)
	assert.Equal(t, 1, res.Invalid)

	require.Len(t, res.SampleInvalidRows, 1)
	detail, ok := res.SampleInvalidRows[FewerFieldsThanExpected]
	require.True(t, ok, "expected a sample for FewerFieldsThanExpected")
	// Line 5 is the short row's physical line, counting the comment and the
	// two whitespace lines above it. Were the whitespace reported rather than
	// skipped, this sample would instead be the empty one-field record on
	// line 3.
	assert.Equal(t, 5, detail.Line)
	assert.Equal(t, []string{"2.0.0.0/24", "NL", "NL-NH", "Amsterdam"}, detail.Fields)
}

// TestProcessGeofeedWhitespaceOnlyFeedIsEmpty covers a feed holding nothing
// but comment and whitespace lines. Skipped records must not count toward
// Total, so this feed is empty rather than a feed full of invalid rows.
func TestProcessGeofeedWhitespaceOnlyFeedIsEmpty(t *testing.T) {
	res, err := ProcessGeofeed(
		"testdata/whitespace-only.csv",
		"testdata/GeoIP2-City-Test.mmdb",
		"",
		Options{},
	)
	require.NoError(t, err)
	assert.Equal(t, FailureEmpty, res.Failure)
	assert.Equal(t, 0, res.Total)
	assert.Equal(t, 0, res.Invalid)
	assert.Empty(t, res.SampleInvalidRows)
	assert.Empty(t, res.FailureDiagnostic)
}

func TestIsBlank(t *testing.T) {
	tests := []struct {
		name string
		row  []string
		want bool
	}{
		{"no fields", nil, true},
		{"one empty field", []string{""}, true},
		{"one whitespace field", []string{" \t "}, true},
		{"one field with content", []string{"1.0.0.0/24"}, false},
		// A lone comma parses as two empty fields. The author did write a
		// row there, just an unusable one, so it stays an invalid row rather
		// than being treated as a blank line.
		{"two empty fields", []string{"", ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isBlank(tt.row))
		})
	}
}

// TestSampleInvalidRowFieldsSurviveReuseRecord guards against Fields
// aliasing csv.Reader's reused backing array, at both population sites. Each
// fixture has three same-type invalid rows with distinct field values; only
// the first of each is captured (first-write-wins), but two more Read calls
// happen afterward, each overwriting the same backing array since
// ReuseRecord is set. If Fields held a view into that array instead of a
// clone, the captured sample would now read back as the third row's
// content.
func TestSampleInvalidRowFieldsSurviveReuseRecord(t *testing.T) {
	t.Run("verifyCorrection site", func(t *testing.T) {
		res, err := ProcessGeofeed(
			"testdata/reuse-record-empty-network-repeated.csv",
			"testdata/GeoIP2-City-Test.mmdb",
			"",
			Options{},
		)
		require.NoError(t, err)
		require.Equal(t, FailureInvalidRows, res.Failure)

		detail, ok := res.SampleInvalidRows[EmptyNetwork]
		require.True(t, ok, "expected a sample detail for EmptyNetwork")

		assert.Equal(t, []string{"", "AAAA", "BBBB", "CCCC", "DDDD"}, detail.Fields)
	})

	t.Run("too-few-fields site", func(t *testing.T) {
		res, err := ProcessGeofeed(
			"testdata/reuse-record-short-row-repeated.csv",
			"testdata/GeoIP2-City-Test.mmdb",
			"",
			Options{},
		)
		require.NoError(t, err)
		require.Equal(t, FailureInvalidRows, res.Failure)

		detail, ok := res.SampleInvalidRows[FewerFieldsThanExpected]
		require.True(t, ok, "expected a sample detail for FewerFieldsThanExpected")

		assert.Equal(t, []string{"AAAA", "BBBB", "CCCC"}, detail.Fields)
	})
}

func TestProcessGeofeed_NonUTF8(t *testing.T) {
	tests := []struct {
		gf   string
		desc string
	}{
		{
			gf:   "testdata/geofeed-valid-utf16le.csv",
			desc: "UTF-16 LE encoded geofeed",
		},
		{
			gf:   "testdata/geofeed-valid-shiftjis.csv",
			desc: "Shift-JIS encoded geofeed",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			res, err := ProcessGeofeed(
				test.gf,
				"testdata/GeoIP2-City-Test.mmdb",
				"",
				Options{},
			)
			require.NoError(t, err)
			assert.Equal(t, FailureNotUTF8, res.Failure)
			// Nothing is read from a file whose encoding cannot be
			// trusted, so no rows are counted or sampled.
			assert.Equal(t, 0, res.Total)
			assert.Empty(t, res.SampleInvalidRows)
			// FailureNotUTF8 says everything there is to say: there is no
			// offset to report, so no diagnostic is manufactured.
			assert.Empty(t, res.FailureDiagnostic)
		})
	}
}

// TestInvalidRowsHaveDiagnostic asserts that every InvalidRow ProcessGeofeed
// produces has a non-empty Diagnostic. That property, not any one kind's
// exact wording, is what String() depends on: it has no fallback to Reason,
// so a future invalidity kind added without a diagnostic would silently
// render as "line N: " instead of failing a test.
func TestInvalidRowsHaveDiagnostic(t *testing.T) {
	fixtures := []string{
		// FewerFieldsThanExpected.
		"testdata/geofeed-invalid-missing-fields.csv",
		// EmptyNetwork.
		"testdata/geofeed-invalid-empty-network.csv",
		// UnableToParseNetwork.
		"testdata/geofeed-invalid-network.csv",
		// InvalidRegionCode, in strict (default) mode.
		"testdata/geofeed-valid-lax.csv",
	}

	for _, gf := range fixtures {
		t.Run(gf, func(t *testing.T) {
			res, err := ProcessGeofeed(
				gf,
				"testdata/GeoIP2-City-Test.mmdb",
				"",
				Options{},
			)
			require.NoError(t, err)
			require.Equal(t, FailureInvalidRows, res.Failure)
			require.NotEmpty(t, res.SampleInvalidRows, "expected at least one sample")

			for invType, detail := range res.SampleInvalidRows {
				assert.NotEmpty(
					t,
					detail.Diagnostic,
					"invalidity type %s has an empty Diagnostic",
					invType,
				)
			}
		})
	}
}

func TestInvalidRowString(t *testing.T) {
	tests := []struct {
		name string
		row  InvalidRow
		want string
	}{
		{
			name: "zero value",
			row:  InvalidRow{},
			want: "line 0: ",
		},
		{
			// A Reason without a Diagnostic must not fall back to Reason:
			// that would make an engineer-facing renderer silently emit
			// customer-facing text instead.
			name: "empty Diagnostic does not fall back to Reason",
			row: InvalidRow{
				Line:   7,
				Type:   EmptyNetwork,
				Reason: "The network field is empty.",
			},
			want: "line 7: ",
		},
		{
			name: "FewerFieldsThanExpected",
			row: InvalidRow{
				Line:       1,
				Type:       FewerFieldsThanExpected,
				Diagnostic: "expected 5 fields but got 4, row: 'a,b,c,d'",
			},
			want: "line 1: expected 5 fields but got 4, row: 'a,b,c,d'",
		},
		{
			name: "EmptyNetwork",
			row: InvalidRow{
				Line:       2,
				Type:       EmptyNetwork,
				Diagnostic: "network field is empty, row: ',,,,'",
			},
			want: "line 2: network field is empty, row: ',,,,'",
		},
		{
			name: "UnableToParseNetwork",
			row: InvalidRow{
				Line:       3,
				Type:       UnableToParseNetwork,
				Diagnostic: "unable to parse network foo: bar",
			},
			want: "line 3: unable to parse network foo: bar",
		},
		{
			name: "InvalidRegionCode",
			row: InvalidRow{
				Line:       6,
				Type:       InvalidRegionCode,
				Diagnostic: "invalid ISO 3166-2 region code format in strict (default) mode, row: 'a,b,c,d,e'",
			},
			want: "line 6: invalid ISO 3166-2 region code format in strict (default) mode, row: 'a,b,c,d,e'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// want never mentions Type even though every non-zero case sets
			// a different one: String() must not include it.
			assert.Equal(t, tt.want, tt.row.String())
		})
	}
}

// TestInvalidRowStringOnMapValue proves String() works when called on a
// map value, which is not addressable. A pointer receiver would fail to
// compile against this call, since Go cannot take the address of a map
// value to satisfy a pointer-receiver method.
func TestInvalidRowStringOnMapValue(t *testing.T) {
	m := map[RowInvalidity]InvalidRow{
		EmptyNetwork: {Line: 1, Diagnostic: "network field is empty, row: ',,,,'"},
	}

	got := m[EmptyNetwork].String()
	assert.Equal(t, "line 1: network field is empty, row: ',,,,'", got)
}

// TestProcessGeofeedCityDatabaseLookupFailure covers the city-database
// lookup-failure path: MaxMind-DB-test-ipv4-24.mmdb is IPv4-only, so
// Reader.Lookup returns an error for any IPv6 address, and that error
// reaches DecodePath verbatim. geofeed-valid.csv's first row is an IPv6
// network, so it triggers this on the very first row processed.
func TestProcessGeofeedCityDatabaseLookupFailure(t *testing.T) {
	geofeedData, readErr := os.ReadFile("testdata/geofeed-valid.csv")
	require.NoError(t, readErr)
	firstLine, _, _ := strings.Cut(string(geofeedData), "\n")
	// If this fails, the fixture's first row no longer exercises an IPv6
	// lookup against an IPv4-only database, and the rest of this test is
	// silently exercising nothing.
	require.Contains(
		t,
		firstLine,
		"2a02:ecc0::/29",
		"fixture's first row must still be an IPv6 network",
	)

	res, err := ProcessGeofeed(
		"testdata/geofeed-valid.csv",
		"testdata/MaxMind-DB-test-ipv4-24.mmdb",
		"",
		Options{},
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDatabaseLookup)
	// Contains, not exact equality: the wording past this point belongs to
	// maxminddb-golang, and a library or Go upgrade could reword it.
	assert.Contains(t, err.Error(), "2a02:ecc0::/29")
	assert.Contains(t, err.Error(), "IPv6")
	// An error is not a verdict on the geofeed: the rows were never
	// evaluated, so Failure must stay FailureNone rather than blame the
	// feed for our database being unusable.
	assert.Equal(t, FailureNone, res.Failure)
}

// TestProcessGeofeedISPDatabaseLookupFailure covers the ISP-database
// lookup-failure path, the first test in this repo to pass a non-empty
// ispFilename. The city database (GeoIP2-City-Test.mmdb) resolves the
// IPv6 network in geofeed-valid.csv's first row successfully; only the ISP
// database (MaxMind-DB-test-ipv4-24.mmdb, IPv4-only) fails to look it up.
func TestProcessGeofeedISPDatabaseLookupFailure(t *testing.T) {
	geofeedData, readErr := os.ReadFile("testdata/geofeed-valid.csv")
	require.NoError(t, readErr)
	firstLine, _, _ := strings.Cut(string(geofeedData), "\n")
	require.Contains(
		t,
		firstLine,
		"2a02:ecc0::/29",
		"fixture's first row must still be an IPv6 network",
	)

	_, err := ProcessGeofeed(
		"testdata/geofeed-valid.csv",
		"testdata/GeoIP2-City-Test.mmdb",
		"testdata/MaxMind-DB-test-ipv4-24.mmdb",
		Options{},
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDatabaseLookup)
	assert.Contains(t, err.Error(), "2a02:ecc0::/29")
	assert.Contains(t, err.Error(), "IPv6")
}

// TestProcessGeofeedISPAugmentation covers the ISP augmentation block
// (asnCounts and the AS Number/AS Name/ISP Name diff lines), which had no
// coverage at all before ProcessGeofeedISPDatabaseLookupFailure -- and that
// test never reaches this code, since it errors out of the lookup.
//
// geofeed-isp.csv's row is deliberately bogus in its country, region,
// city, and postal code: ::1.128.0.0/107 is not present in
// GeoIP2-City-Test.mmdb, so every one of those fields decodes to its zero
// value, and the bogus values guarantee every comparison mismatches --
// this is what guarantees foundDiff, and therefore the AS/ISP lines,
// without depending on real data in a fixture this small. ZZ-ZZ carries a
// hyphen, so it passes strict-mode region-code validation and the row
// survives to reach the augmentation code at all.
func TestProcessGeofeedISPAugmentation(t *testing.T) {
	res, err := ProcessGeofeed(
		"testdata/geofeed-isp.csv",
		"testdata/GeoIP2-City-Test.mmdb",
		"testdata/GeoIP2-ISP-Test.mmdb",
		Options{},
	)
	require.NoError(t, err)
	assert.Equal(t, FailureNone, res.Failure)
	assert.Equal(t, 1, res.Total)
	assert.Equal(t, 1, res.Differences)
	assert.Equal(t, 0, res.Invalid)

	// ::1.128.0.0/107 in GeoIP2-ISP-Test.mmdb carries ASN 1221, "Telstra Pty
	// Ltd", ISP "Telstra Internet".
	assert.Equal(t, map[uint]int{1221: 1}, res.ASNCounts)

	require.Len(t, res.Diffs, 1)
	assert.Contains(t, res.Diffs[0], "AS Number: 1221")
	assert.Contains(t, res.Diffs[0], "AS Name: Telstra Pty Ltd")
	assert.Contains(t, res.Diffs[0], "ISP Name: Telstra Internet")
}

// TestProcessGeofeedUnopenableCityDatabase covers the City MMDB
// open-failure path. A database that can't be opened at all is the same
// kind of MaxMind-side fault as a failed lookup, so ErrDatabaseLookup must
// wrap this too, not just the two decode-failure sites.
func TestProcessGeofeedUnopenableCityDatabase(t *testing.T) {
	_, err := ProcessGeofeed(
		"testdata/geofeed-valid.csv",
		"testdata/does-not-exist.mmdb",
		"",
		Options{},
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDatabaseLookup)
}

// TestProcessGeofeedUnopenableISPDatabase covers the ISP MMDB open-failure
// path specifically -- a different return site than the City one above,
// so it needs its own proof that ErrDatabaseLookup wraps it too.
func TestProcessGeofeedUnopenableISPDatabase(t *testing.T) {
	_, err := ProcessGeofeed(
		"testdata/geofeed-valid.csv",
		"testdata/GeoIP2-City-Test.mmdb",
		"testdata/does-not-exist.mmdb",
		Options{},
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDatabaseLookup)
}

// TestProcessGeofeedUnreadableCSV covers the CSV parse-failure path.
// Malformed quoting is the geofeed's fault, so it is a Failure and not an
// error, and the reader cannot resynchronize afterward: the counts stop at
// the last row read before the offending line.
func TestProcessGeofeedUnreadableCSV(t *testing.T) {
	res, err := ProcessGeofeed(
		"testdata/malformed-quoting.csv",
		"testdata/GeoIP2-City-Test.mmdb",
		"",
		Options{},
	)
	require.NoError(t, err)
	assert.Equal(t, FailureUnreadableCSV, res.Failure)
	// Only the first data row (file line 2) was read; the malformed line 3
	// aborts parsing, so line 4's valid row is never seen.
	assert.Equal(t, 1, res.Total)
	assert.Equal(t, 0, res.Invalid)
	// The offending line is not a row that failed validation -- it never
	// parsed into a row at all -- so it must not appear as a sample.
	assert.Empty(t, res.SampleInvalidRows)
	// The whole diagnostic, not a substring: line 3, column 15 is where the
	// stray character after the closing quote sits, and that position is the
	// only record of where the trouble is, since no row was produced.
	assert.Equal(
		t,
		`unable to read next row in testdata/malformed-quoting.csv: `+
			`parse error on line 3, column 15: extraneous or missing " in quoted-field`,
		res.FailureDiagnostic,
	)
}

// TestProcessGeofeedUnreadableCSVHidingFilePaths proves
// HideFilePathsInErrorMessages covers FailureDiagnostic too. The geofeed path
// is a build-host path, and this diagnostic is persisted by consumers, so the
// option would be pointless if the one populated diagnostic ignored it.
func TestProcessGeofeedUnreadableCSVHidingFilePaths(t *testing.T) {
	res, err := ProcessGeofeed(
		"testdata/malformed-quoting.csv",
		"testdata/GeoIP2-City-Test.mmdb",
		"",
		Options{HideFilePathsInErrorMessages: true},
	)
	require.NoError(t, err)
	assert.Equal(t, FailureUnreadableCSV, res.Failure)
	assert.Equal(
		t,
		`unable to read next row: `+
			`parse error on line 3, column 15: extraneous or missing " in quoted-field`,
		res.FailureDiagnostic,
	)
	// Named separately from the equality above: this is the property the
	// option exists for, and it should fail loudly if the wording changes
	// but the path creeps back in.
	assert.NotContains(t, res.FailureDiagnostic, "testdata")
}

// TestResultZeroValueFailureIsNone guards against FailureReason's zero value
// meaning anything other than "passed". A Result a caller forgot to populate
// must not claim the geofeed failed, which is what makes reporting failures
// as values safe.
func TestResultZeroValueFailureIsNone(t *testing.T) {
	var zero Result
	assert.Equal(t, FailureNone, zero.Failure)
}

func TestFailureReasonString(t *testing.T) {
	tests := []struct {
		reason FailureReason
		want   string
	}{
		{FailureNone, "FailureNone"},
		{FailureNotUTF8, "FailureNotUTF8"},
		{FailureEmpty, "FailureEmpty"},
		{FailureInvalidRows, "FailureInvalidRows"},
		{FailureUnreadableCSV, "FailureUnreadableCSV"},
		{FailureReason(99), "UnknownFailureReason"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.reason.String())
		})
	}
}

// TestProcessGeofeedCorruptCityDatabase covers a database that exists but
// isn't a valid MMDB at all, as opposed to simply missing -- a cheap
// second trigger for the same open-failure path, reusing an existing
// non-MMDB fixture rather than adding a new binary one.
func TestProcessGeofeedCorruptCityDatabase(t *testing.T) {
	_, err := ProcessGeofeed(
		"testdata/geofeed-valid.csv",
		"testdata/geofeed-valid.csv",
		"",
		Options{},
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDatabaseLookup)
}
