package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/maxmind/mm-geofeed-verifier/v4/verify"
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
				db: "",
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
			require.NoError(t, err, "parseFlags ran without error")
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
			[]string{"-db", "file.mdb"},
			"Path to local geofeed file",
			"-gf is required",
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

func TestFormatInvalidRows(t *testing.T) {
	// Keys inserted out of RowInvalidity order, to prove the output order
	// comes from sorting rather than accidentally matching insertion or map
	// iteration order.
	rows := map[verify.RowInvalidity]verify.InvalidRow{
		verify.InvalidRegionCode: {
			Line:       3,
			Diagnostic: "invalid ISO 3166-2 region code format in strict (default) mode, row: 'a,b,c,d,e'",
		},
		verify.EmptyNetwork: {
			Line:       1,
			Diagnostic: "network field is empty, row: ',,,,'",
		},
		// FewerFieldsThanExpected's diagnostic already ends in "row:
		// '...'": formatInvalidRows must not wrap it in a second layer of
		// quotes.
		verify.FewerFieldsThanExpected: {
			Line:       2,
			Diagnostic: "expected 5 fields but got 4, row: 'a,b,c,d'",
		},
	}

	got := formatInvalidRows(rows)

	// Sorted by RowInvalidity's underlying int value (its iota order), not
	// map iteration order.
	want := []string{
		"FewerFieldsThanExpected: line 2: expected 5 fields but got 4, row: 'a,b,c,d'",
		"EmptyNetwork: line 1: network field is empty, row: ',,,,'",
		"InvalidRegionCode: line 3: invalid ISO 3166-2 region code format " +
			"in strict (default) mode, row: 'a,b,c,d,e'",
	}
	assert.Equal(t, want, got)

	for _, line := range got {
		assert.NotContains(t, line, "''", "line has doubled quotes: %q", line)
	}
}
