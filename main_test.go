package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/maxmind/mm-geofeed-verifier/v5/verify"
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

// TestRun drives run end to end. Nothing else does, so the distinction this
// program exists to make -- an unusable database is our problem, an invalid
// geofeed is the feed's -- is only pinned here. Swapping those two branches
// leaves every other test passing.
func TestRun(t *testing.T) {
	const (
		cityDB     = "verify/testdata/GeoIP2-City-Test.mmdb"
		absentDB   = "verify/testdata/does-not-exist.mmdb"
		validFeed  = "verify/testdata/geofeed-valid.csv"
		shortRows  = "verify/testdata/geofeed-invalid-missing-fields.csv"
		badQuoting = "verify/testdata/malformed-quoting.csv"
	)

	tests := []struct {
		name string
		args []string
		// wantErrContains holds substrings the returned error must have, or
		// is empty when the geofeed is expected to pass.
		wantErrContains []string
		wantDatabaseErr bool
	}{
		{
			name:            "unusable database",
			args:            []string{"-gf", validFeed, "-db", absentDB},
			wantErrContains: []string{"MMDB unavailable"},
			wantDatabaseErr: true,
		},
		{
			name:            "geofeed with invalid rows",
			args:            []string{"-gf", shortRows, "-db", cityDB},
			wantErrContains: []string{"failed verification", "RFC 8805"},
		},
		{
			name: "geofeed the parser cannot read",
			args: []string{"-gf", badQuoting, "-db", cityDB},
			// The parse location reaches the user only through
			// FailureDiagnostic, so its absence would be silent.
			wantErrContains: []string{
				"could not be parsed as CSV",
				"line 3, column 15",
			},
		},
		{
			name: "valid geofeed",
			args: []string{"-gf", validFeed, "-db", cityDB},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := run("mm-geofeed-verifier", test.args)

			if len(test.wantErrContains) == 0 {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			for _, want := range test.wantErrContains {
				assert.Contains(t, err.Error(), want)
			}

			// A geofeed's own failure must not carry the database sentinel: a
			// consumer keys its retry-versus-report decision on it.
			if test.wantDatabaseErr {
				require.ErrorIs(t, err, verify.ErrDatabaseLookup)
			} else {
				require.NotErrorIs(t, err, verify.ErrDatabaseLookup)
			}
		})
	}
}
