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
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]string{},
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
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]string{},
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
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]string{},
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
				Total:             3,
				Differences:       2,
				SampleInvalidRows: map[RowInvalidity]string{},
			},
			laxMode: false,
		},
		{
			gf: "test_data/empty.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			c: CheckResult{
				Total:             0,
				SampleInvalidRows: map[RowInvalidity]string{},
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
					UnableToParseNetwork: "line 1: unable to parse network 2a02:/29: invalid CIDR address: 2a02:/29",
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
			},
			em:      ErrInvalidGeofeed,
			laxMode: false,
		},
		{
			gf: "test_data/empty.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			c: CheckResult{
				Total:             0,
				SampleInvalidRows: map[RowInvalidity]string{},
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
