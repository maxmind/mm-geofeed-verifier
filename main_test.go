package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/maxmind/mm-geofeed-verifier/v2/verify"
)

type parseFlagsCorrectTest struct {
	args []string
	conf config
}

func TestParseFlagsCorrect(t *testing.T) {
	tests := []parseFlagsCorrectTest{
		{
			[]string{"-gf", "geofeed.csv"},
			config{
				gf: "geofeed.csv",
				db: "/usr/local/share/GeoIP/GeoIP2-City.mmdb",
			},
		},
		{
			[]string{"-gf", "geofeed.csv", "-db", "file.mmdb"},
			config{
				gf: "geofeed.csv",
				db: "file.mmdb",
			},
		},
		{
			[]string{"-db", "file.mmdb", "-gf", "geofeed.csv"},
			config{
				gf: "geofeed.csv",
				db: "file.mmdb",
			},
		},
		{
			[]string{"--lax", "-db", "file.mmdb", "-gf", "geofeed.csv"},
			config{
				gf:      "geofeed.csv",
				db:      "file.mmdb",
				laxMode: true,
			},
		},
		{
			[]string{"-db", "file.mmdb", "-lax=true", "-gf", "geofeed.csv"},
			config{
				gf:      "geofeed.csv",
				db:      "file.mmdb",
				laxMode: true,
			},
		},
		{
			[]string{"-db", "file.mmdb", "-gf", "geofeed.csv", "--lax=false"},
			config{
				gf:      "geofeed.csv",
				db:      "file.mmdb",
				laxMode: false,
			},
		},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			conf, output, err := parseFlags("program", test.args)
			assert.NoError(t, err, "parseFlags ran without error")
			assert.Empty(t, output, "parseFlags ran without output")
			assert.Equal(t, test.conf, *conf, "parseFlags produced expected config")
		})
	}
}

func TestParseFlagsUsage(t *testing.T) {
	usageArgs := []string{"-help", "-h", "--help"}

	for _, arg := range usageArgs {
		t.Run(arg, func(t *testing.T) {
			conf, output, err := parseFlags("program", []string{arg})
			assert.Equal(t, flag.ErrHelp, err)
			assert.Nil(t, conf, "there should be no config set")
			assert.Contains(t, output, "Usage of", "output contains usage info")
		})
	}
}

type parseFlagsErrorTest struct {
	args   []string
	output string
	errmsg string
}

func TestParseFlagsError(t *testing.T) {
	tests := []parseFlagsErrorTest{
		{
			[]string{},
			"Path to local geofeed file",
			"-gf is required",
		},
		{
			[]string{"-db", ""},
			"Path to local geofeed file",
			"-gf is required and -db can not be an emptry string",
		},
		{
			[]string{"-db", "file.mdb"},
			"Path to local geofeed file",
			"-gf is required",
		},
		{
			[]string{"-gf", "geofeed.csv", "-db", ""},
			"Path to local geofeed file",
			"-db is required",
		},
	}

	for _, test := range tests {
		t.Run(
			strings.Join(test.args, " "), func(t *testing.T) {
				_, output, err := parseFlags("program", test.args)
				assert.Contains(
					t,
					output,
					test.output,
					"output contains usage info: '%s'", test.output,
				)
				assert.EqualError(
					t,
					err,
					test.errmsg,
					"got expected error message: '%s'", test.errmsg,
				)
			},
		)
	}
}

type processGeofeedTest struct {
	gf      string
	db      string
	dl      []string
	c       verify.Counts
	em      string
	laxMode bool
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
			c: verify.Counts{
				Total:       3,
				Differences: 2,
			},
			em:      "",
			laxMode: false,
		},
		{
			gf: "test_data/geofeed-valid.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: verify.Counts{
				Total:       3,
				Differences: 2,
			},
			em:      "",
			laxMode: true,
		},
		{
			gf: "test_data/geofeed-valid-lax.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: verify.Counts{
				Total:       3,
				Differences: 2,
			},
			em:      "",
			laxMode: true,
		},
		{
			gf: "test_data/geofeed-valid-optional-fields.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			dl: []string{
				"Found a potential improvement: '2a02:ecc0::/29",
				"current postal code: '34021'\t\tsuggested postal code: '1060'",
			},
			c: verify.Counts{
				Total:       3,
				Differences: 2,
			},
			em:      "",
			laxMode: false,
		},
	}

	// Testing the full content of the difference explanation strings is likely to be
	// tedious and brittle, so we will just check for some substrings.
	for _, test := range goodTests {
		t.Run(
			test.gf+" "+test.db, func(t *testing.T) {
				c, dl, _, err := verify.ProcessGeofeed(test.gf, test.db, "", test.laxMode)
				assert.NoError(t, err, "processGeofeed ran without error")
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
			gf:      "test_data/geofeed-invalid-missing-fields.csv",
			db:      "test_data/GeoIP2-City-Test.mmdb",
			dl:      []string{},
			c:       verify.Counts{},
			em:      "saw fewer than the expected 5 fields at line 1: '2a02:ecc0::/29,US,US-NJ,Parsippany'",
			laxMode: false,
		},
		{
			gf:      "test_data/geofeed-invalid-empty-network.csv",
			db:      "test_data/GeoIP2-City-Test.mmdb",
			dl:      []string{},
			c:       verify.Counts{},
			em:      "line 2: network field is empty",
			laxMode: false,
		},
		{
			gf:      "test_data/geofeed-invalid-network.csv",
			db:      "test_data/GeoIP2-City-Test.mmdb",
			dl:      []string{},
			c:       verify.Counts{},
			em:      "line 1: unable to parse network 2a02:/29: invalid CIDR address: 2a02:/29",
			laxMode: false,
		},
		{
			gf: "test_data/geofeed-valid-lax.csv",
			db: "test_data/GeoIP2-City-Test.mmdb",
			dl: []string{},
			c:  verify.Counts{},
			em: "line 1: invalid ISO 3166-2 region code format in strict (default) mode, line: " +
				"'2a02:ecc0::/29,US,NJ,Parsippany,'",
			laxMode: false,
		},
	}

	for _, test := range badTests {
		t.Run(
			test.gf+" "+test.db, func(t *testing.T) {
				_, _, _, err := verify.ProcessGeofeed(test.gf, test.db, "", test.laxMode)
				assert.EqualError(
					t,
					err,
					test.em,
					"got expected error: %s", test.em,
				)
			},
		)
	}
}
