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
			gf: "test_data/geofeed-valid.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: CheckResult{
				Total:                   3,
				Differences:             2,
				SampleInvalidRows:       map[RowInvalidity]string{},
				SampleInvalidRowDetails: map[RowInvalidity]InvalidRow{},
			},
			laxMode: false,
		},
		{
			gf: "test_data/geofeed-valid.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: CheckResult{
				Total:                   3,
				Differences:             2,
				SampleInvalidRows:       map[RowInvalidity]string{},
				SampleInvalidRowDetails: map[RowInvalidity]InvalidRow{},
			},
			laxMode: true,
		},
		{
			gf: "test_data/geofeed-valid-lax.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: CheckResult{
				Total:                   3,
				Differences:             2,
				SampleInvalidRows:       map[RowInvalidity]string{},
				SampleInvalidRowDetails: map[RowInvalidity]InvalidRow{},
			},
			laxMode: true,
		},
		{
			gf: "test_data/geofeed-valid-optional-fields.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: CheckResult{
				Total:                   3,
				Differences:             2,
				SampleInvalidRows:       map[RowInvalidity]string{},
				SampleInvalidRowDetails: map[RowInvalidity]InvalidRow{},
			},
			laxMode: false,
		},
		{
			gf: "test_data/geofeed-valid-utf8-bom.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: CheckResult{
				Total:                   3,
				Differences:             2,
				SampleInvalidRows:       map[RowInvalidity]string{},
				SampleInvalidRowDetails: map[RowInvalidity]InvalidRow{},
			},
			laxMode: false,
		},
		{
			gf: "test_data/empty.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			c: CheckResult{
				Total:                   0,
				SampleInvalidRows:       map[RowInvalidity]string{},
				SampleInvalidRowDetails: map[RowInvalidity]InvalidRow{},
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
			gf: "test_data/geofeed-invalid-missing-fields.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			c: CheckResult{
				Total:       2,
				Differences: 0,
				Invalid:     2,
				SampleInvalidRows: map[RowInvalidity]string{
					FewerFieldsThanExpected: "line 1: expected 5 fields but got 4, " +
						"row: '2a02:ecc0::/29,US,US-NJ,Parsippany'",
				},
				SampleInvalidRowDetails: map[RowInvalidity]InvalidRow{
					FewerFieldsThanExpected: {
						Line:   1,
						Type:   FewerFieldsThanExpected,
						Row:    "2a02:ecc0::/29,US,US-NJ,Parsippany",
						Reason: "expected 5 fields but got 4",
					},
				},
			},
			em:      ErrInvalidGeofeed,
			laxMode: false,
		},
		{
			gf: "test_data/geofeed-invalid-empty-network.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			c: CheckResult{
				Total:       2,
				Differences: 1,
				Invalid:     1,
				SampleInvalidRows: map[RowInvalidity]string{
					EmptyNetwork: "line 2: network field is empty, row: ',,,,'",
				},
				SampleInvalidRowDetails: map[RowInvalidity]InvalidRow{
					EmptyNetwork: {
						Line:   2,
						Type:   EmptyNetwork,
						Row:    ",,,,",
						Reason: "network field is empty, row: ',,,,'",
					},
				},
			},
			em:      ErrInvalidGeofeed,
			laxMode: false,
		},
		{
			gf: "test_data/geofeed-invalid-network.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			c: CheckResult{
				Total:       2,
				Differences: 1,
				Invalid:     1,
				SampleInvalidRows: map[RowInvalidity]string{
					UnableToParseNetwork: `line 1: unable to parse network 2a02:/29: netip.ParsePrefix("2a02:/29"): ParseAddr("2a02:"): colon must be followed by more characters (at ":")`,
				},
				SampleInvalidRowDetails: map[RowInvalidity]InvalidRow{
					UnableToParseNetwork: {
						Line: 1,
						Type: UnableToParseNetwork,
						Row:  "2a02:/29,,,,",
						Reason: `unable to parse network 2a02:/29: netip.ParsePrefix("2a02:/29"): ` +
							`ParseAddr("2a02:"): colon must be followed by more characters (at ":")`,
					},
				},
			},
			em:      ErrInvalidGeofeed,
			laxMode: false,
		},
		{
			// Geofeed that is valid in lax mode should not be valid if laxMode == true.
			gf: "test_data/geofeed-valid-lax.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			c: CheckResult{
				Total:       3,
				Differences: 1,
				Invalid:     2,
				SampleInvalidRows: map[RowInvalidity]string{
					InvalidRegionCode: "line 1: invalid ISO 3166-2 region code format " +
						"in strict (default) mode, row: '2a02:ecc0::/29,US,NJ,Parsippany,'",
				},
				SampleInvalidRowDetails: map[RowInvalidity]InvalidRow{
					// Row keeps the trailing whitespace inside the country and
					// city fields: it is a snapshot from before verifyCorrection
					// trims its (aliased) copy, unlike the message text above.
					InvalidRegionCode: {
						Line: 1,
						Type: InvalidRegionCode,
						Row:  "2a02:ecc0::/29 ,US,NJ ,Parsippany ,",
						Reason: "invalid ISO 3166-2 region code format in strict (default) mode, " +
							"row: '2a02:ecc0::/29,US,NJ,Parsippany,'",
					},
				},
			},
			em:      ErrInvalidGeofeed,
			laxMode: false,
		},
		{
			gf: "test_data/empty.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			c: CheckResult{
				Total:                   0,
				SampleInvalidRows:       map[RowInvalidity]string{},
				SampleInvalidRowDetails: map[RowInvalidity]InvalidRow{},
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
			"test_data/geofeed-valid.csv",
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
			"test_data/geofeed-invalid-missing-fields.csv",
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
			"test_data/geofeed-valid-lax.csv",
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
			"test_data/geofeed-valid-lax.csv",
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
			"test_data/geofeed-valid.csv",
			"",
			"",
			Options{},
		)
		require.NoError(t, err, "processGeofeed ran without error when mmdbFilename is empty")
	})

	t.Run("missing mmdb path still errors", func(t *testing.T) {
		_, _, _, err := ProcessGeofeed(
			"test_data/geofeed-valid.csv",
			"test_data/does-not-exist.mmdb",
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

func TestSampleInvalidRowDetailsReportsFileLine(t *testing.T) {
	counts, _, _, err := ProcessGeofeed(
		"test_data/comments-then-short-row.csv",
		"test_data/GeoIP2-City-Test.mmdb",
		"",
		Options{LaxMode: true, HideFilePathsInErrorMessages: true},
	)
	require.ErrorIs(t, err, ErrInvalidGeofeed)

	detail, ok := counts.SampleInvalidRowDetails[FewerFieldsThanExpected]
	require.True(t, ok, "expected a sample detail for FewerFieldsThanExpected")

	// The short row is on file line 5. c.Total reports 2 at this point in
	// processing (it counts data rows: lines 3 and 5, skipping the three
	// comment lines above: lines 1, 2, and 4), as shown by the legacy
	// message text below, which embeds c.Total rather than the file line.
	assert.Equal(t, 5, detail.Line)
	assert.Equal(t, FewerFieldsThanExpected, detail.Type)
	assert.Equal(t, "2.0.0.0/24,NL,NL-NH,Amsterdam", detail.Row)
	assert.Equal(
		t,
		"line 2: expected 5 fields but got 4, row: '2.0.0.0/24,NL,NL-NH,Amsterdam'",
		counts.SampleInvalidRows[FewerFieldsThanExpected],
	)

	// The two maps must never disagree about which row is the sample.
	assert.Contains(t, counts.SampleInvalidRows[FewerFieldsThanExpected], detail.Row)
}

// TestSampleInvalidRowDetailsReportsFileLineForVerifyCorrection covers the
// second SampleInvalidRowDetails population site: rows that reach
// verifyCorrection (as opposed to the too-few-fields site covered by
// TestSampleInvalidRowDetailsReportsFileLine above). Reverting this site's
// Line back to the comment-skipping row counter must fail this test.
func TestSampleInvalidRowDetailsReportsFileLineForVerifyCorrection(t *testing.T) {
	counts, _, _, err := ProcessGeofeed(
		"test_data/comments-then-empty-network.csv",
		"test_data/GeoIP2-City-Test.mmdb",
		"",
		Options{},
	)
	require.ErrorIs(t, err, ErrInvalidGeofeed)

	detail, ok := counts.SampleInvalidRowDetails[EmptyNetwork]
	require.True(t, ok, "expected a sample detail for EmptyNetwork")

	// The empty-network row is on file line 5. c.Total would report 2,
	// because it counts data rows (lines 3 and 5) and skips the three
	// comment lines above (lines 1, 2, and 4).
	assert.Equal(t, 5, detail.Line)
	assert.Equal(t, EmptyNetwork, detail.Type)
	assert.Equal(t, ",,,,", detail.Row)
	assert.Equal(t, 2, counts.Total)

	// The two maps must never disagree about which row is the sample.
	assert.Contains(t, counts.SampleInvalidRows[EmptyNetwork], detail.Row)
}

func TestProcessGeofeed_NonUTF8(t *testing.T) {
	tests := []struct {
		gf   string
		desc string
	}{
		{
			gf:   "test_data/geofeed-valid-utf16le.csv",
			desc: "UTF-16 LE encoded geofeed",
		},
		{
			gf:   "test_data/geofeed-valid-shiftjis.csv",
			desc: "Shift-JIS encoded geofeed",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			_, _, _, err := ProcessGeofeed(
				test.gf,
				"test_data/GeoIP2-City-Test.mmdb",
				"",
				Options{},
			)
			require.ErrorIs(t, err, ErrNotUTF8)
		})
	}
}
