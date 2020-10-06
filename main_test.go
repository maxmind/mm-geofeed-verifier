package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
	}

	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			conf, output, err := parseFlags("program", test.args)
			assert.NoError(t, err, "parseFlags ran without error")
			assert.Empty(t, output, "parseFlags ran without output")
			assert.Equal(t, *conf, test.conf, "parseFlags produced expected config")
		})
	}
}

func TestParseFlagsUsage(t *testing.T) {
	var usageArgs = []string{"-help", "-h", "--help"}

	for _, arg := range usageArgs {
		t.Run(arg, func(t *testing.T) {
			conf, output, err := parseFlags("program", []string{arg})
			assert.Equal(t, err, flag.ErrHelp)
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
			strings.Join(test.args, ""), func(t *testing.T) {
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
