package verify

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type processGeofeedTest struct {
	gf      string
	db      string
	dl      []string
	c       CheckResult
	em      error
	laxMode bool
	emptyOK bool
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
			c: CheckResult{
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
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
			c: CheckResult{
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
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
			c: CheckResult{
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
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
			c: CheckResult{
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
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
			c: CheckResult{
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
			},
			laxMode: false,
		},
		{
			gf: "testdata/empty.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: CheckResult{
				Total:             0,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
			},
			emptyOK: true,
		},
	}

	// Testing the full content of the difference explanation strings is likely to be
	// tedious and brittle, so we will just check for some substrings.
	for _, test := range goodTests {
		t.Run(
			test.gf+" "+test.db, func(t *testing.T) {
				c, dl, _, err := ProcessGeofeed(
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
						dl[i],
						s,
						"got expected substring: '%s', substring",
					)
				}
				assert.Equal(t, test.c, c, "processGeofeed returned expected results")
			},
		)
	}
}

func TestProcessGeofeed_Invalid(t *testing.T) {
	badTests := []processGeofeedTest{
		{
			gf: "testdata/geofeed-invalid-missing-fields.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: CheckResult{
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
			},
			em:      ErrInvalidGeofeed,
			laxMode: false,
		},
		{
			gf: "testdata/geofeed-invalid-empty-network.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: CheckResult{
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
			},
			em:      ErrInvalidGeofeed,
			laxMode: false,
		},
		{
			gf: "testdata/geofeed-invalid-network.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: CheckResult{
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
			},
			em:      ErrInvalidGeofeed,
			laxMode: false,
		},
		{
			// Geofeed that is valid in lax mode should not be valid if laxMode == true.
			gf: "testdata/geofeed-valid-lax.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: CheckResult{
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
			},
			em:      ErrInvalidGeofeed,
			laxMode: false,
		},
		{
			gf: "testdata/empty.csv",
			db: "testdata/GeoIP2-City-Test.mmdb",
			c: CheckResult{
				Total:             0,
				SampleInvalidRows: map[RowInvalidity]InvalidRow{},
			},
			em:      ErrEmptyGeofeed,
			emptyOK: false,
		},
	}

	for _, test := range badTests {
		t.Run(
			test.gf+" "+test.db, func(t *testing.T) {
				c, _, _, err := ProcessGeofeed(
					test.gf,
					test.db,
					"",
					Options{
						EmptyOK: test.emptyOK,
						LaxMode: test.laxMode,
					},
				)
				require.ErrorIs(
					t,
					err,
					test.em,
					"got expected error: %s", test.em,
				)
				assert.Equal(t, test.c, c)
			},
		)
	}
}

func TestProcessGeofeed_FormatOnly(t *testing.T) {
	t.Run("valid feed, format-only", func(t *testing.T) {
		c, dl, asnCounts, err := ProcessGeofeed(
			"testdata/geofeed-valid.csv",
			"",
			"",
			Options{},
		)
		require.NoError(t, err, "processGeofeed ran without error")
		assert.Equal(t, 3, c.Total, "expected total rows")
		assert.Equal(t, 0, c.Differences, "expected no differences")
		assert.Equal(t, 0, c.Invalid, "expected no invalid rows")
		assert.Empty(t, dl, "expected no diff lines")
		assert.Empty(t, asnCounts, "expected no asn counts")
	})

	t.Run("malformed feed, format-only", func(t *testing.T) {
		c, _, _, err := ProcessGeofeed(
			"testdata/geofeed-invalid-missing-fields.csv",
			"",
			"",
			Options{},
		)
		require.ErrorIs(t, err, ErrInvalidGeofeed, "got expected error")
		assert.Contains(
			t,
			c.SampleInvalidRows,
			FewerFieldsThanExpected,
			"expected a FewerFieldsThanExpected entry",
		)
	})

	t.Run("bad region code in strict mode, format-only", func(t *testing.T) {
		c, _, _, err := ProcessGeofeed(
			"testdata/geofeed-valid-lax.csv",
			"",
			"",
			Options{LaxMode: false},
		)
		require.ErrorIs(t, err, ErrInvalidGeofeed, "got expected error")
		assert.Contains(
			t,
			c.SampleInvalidRows,
			InvalidRegionCode,
			"expected an InvalidRegionCode entry",
		)
		assert.Equal(t, 2, c.Invalid, "expected two invalid rows")
		assert.Equal(t, 0, c.Differences, "expected no differences")
	})

	t.Run("lax mode accepts non-prefixed region codes, format-only", func(t *testing.T) {
		c, dl, asnCounts, err := ProcessGeofeed(
			"testdata/geofeed-valid-lax.csv",
			"",
			"",
			Options{LaxMode: true},
		)
		require.NoError(t, err, "processGeofeed ran without error in lax format-only mode")
		assert.Equal(t, 3, c.Total, "expected total rows")
		assert.Equal(t, 0, c.Invalid, "expected no invalid rows in lax mode")
		assert.Equal(t, 0, c.Differences, "expected no differences")
		assert.Empty(t, dl, "expected no diff lines")
		assert.Empty(t, asnCounts, "expected no asn counts")
	})

	t.Run("empty mmdb path opens no DB", func(t *testing.T) {
		_, _, _, err := ProcessGeofeed(
			"testdata/geofeed-valid.csv",
			"",
			"",
			Options{},
		)
		require.NoError(t, err, "processGeofeed ran without error when mmdbFilename is empty")
	})

	t.Run("missing mmdb path still errors", func(t *testing.T) {
		_, _, _, err := ProcessGeofeed(
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
	counts, _, _, err := ProcessGeofeed(
		"testdata/comments-then-short-row.csv",
		"testdata/GeoIP2-City-Test.mmdb",
		"",
		Options{LaxMode: true, HideFilePathsInErrorMessages: true},
	)
	require.ErrorIs(t, err, ErrInvalidGeofeed)

	detail, ok := counts.SampleInvalidRows[FewerFieldsThanExpected]
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
	counts, _, _, err := ProcessGeofeed(
		"testdata/comments-then-empty-network.csv",
		"testdata/GeoIP2-City-Test.mmdb",
		"",
		Options{},
	)
	require.ErrorIs(t, err, ErrInvalidGeofeed)

	detail, ok := counts.SampleInvalidRows[EmptyNetwork]
	require.True(t, ok, "expected a sample for EmptyNetwork")

	// The empty-network row is on file line 5: Line counts the three
	// comment lines above it (lines 1, 2, and 4); Total, by contrast,
	// counts only data rows (lines 3 and 5), so it is 2 here.
	assert.Equal(t, 5, detail.Line)
	assert.Equal(t, EmptyNetwork, detail.Type)
	assert.Equal(t, []string{"", "", "", "", ""}, detail.Fields)
	assert.Equal(t, 2, counts.Total)
	assert.Equal(t, "network field is empty, row: ',,,,'", detail.Diagnostic)
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
		counts, _, _, err := ProcessGeofeed(
			"testdata/reuse-record-empty-network-repeated.csv",
			"testdata/GeoIP2-City-Test.mmdb",
			"",
			Options{},
		)
		require.ErrorIs(t, err, ErrInvalidGeofeed)

		detail, ok := counts.SampleInvalidRows[EmptyNetwork]
		require.True(t, ok, "expected a sample detail for EmptyNetwork")

		assert.Equal(t, []string{"", "AAAA", "BBBB", "CCCC", "DDDD"}, detail.Fields)
	})

	t.Run("too-few-fields site", func(t *testing.T) {
		counts, _, _, err := ProcessGeofeed(
			"testdata/reuse-record-short-row-repeated.csv",
			"testdata/GeoIP2-City-Test.mmdb",
			"",
			Options{},
		)
		require.ErrorIs(t, err, ErrInvalidGeofeed)

		detail, ok := counts.SampleInvalidRows[FewerFieldsThanExpected]
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
			_, _, _, err := ProcessGeofeed(
				test.gf,
				"testdata/GeoIP2-City-Test.mmdb",
				"",
				Options{},
			)
			require.ErrorIs(t, err, ErrNotUTF8)
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
			counts, _, _, err := ProcessGeofeed(
				gf,
				"testdata/GeoIP2-City-Test.mmdb",
				"",
				Options{},
			)
			require.ErrorIs(t, err, ErrInvalidGeofeed)
			require.NotEmpty(t, counts.SampleInvalidRows, "expected at least one sample")

			for invType, detail := range counts.SampleInvalidRows {
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
