// This script is meant to help verify 'bulk correction' files for submission
// to MaxMind. The files are expected to (mostly) follow the format provided by the RFC at
// https://datatracker.ietf.org/doc/rfc8805/
// Region codes without the country prefix are accepted. eg, 'NY' is allowed, along with
// 'US-NY' for the state of New York in the United States.
// Beyond verifying that the format of the data is correct, the script will also compare
// the corrections against a given MMDB, reporting on how many corrections differ from
// the contents in the database.
package main

import (
	"bytes"
	"cmp"
	"errors"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/maxmind/mm-geofeed-verifier/v5/verify"
)

// This value is set by build scripts. Changing the name of
// the variable should be considered a breaking change.
var version = "unknown"

type config struct {
	gf      string
	db      string
	isp     string
	laxMode bool
	emptyOK bool
}

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	conf, output, err := parseFlags(os.Args[0], os.Args[1:])
	if err != nil {
		fmt.Println(output)
		return err
	}

	if conf.db == "" && conf.isp != "" {
		fmt.Fprintln(os.Stderr, "-isp is ignored without -db")
	}

	res, err := verify.ProcessGeofeed(
		conf.gf,
		conf.db,
		conf.isp,
		verify.Options{LaxMode: conf.laxMode, EmptyOK: conf.emptyOK},
	)
	if err != nil {
		if errors.Is(err, verify.ErrDatabaseLookup) {
			// Name the fault plainly: it's the MMDB, not the geofeed, so
			// this must not read like "unable to process geofeed", which
			// would tell someone their file is bad when it's our database.
			return fmt.Errorf("MMDB unavailable while verifying %s: %w", conf.gf, err)
		}
		return fmt.Errorf("unable to process geofeed %s: %w", conf.gf, err)
	}

	if res.Failure != verify.FailureNone {
		if res.Failure == verify.FailureInvalidRows {
			log.Printf(
				"Found %d invalid rows out of %d rows in total, examples by type:",
				res.Invalid,
				res.Total,
			)
			for _, line := range formatInvalidRows(res.SampleInvalidRows) {
				log.Print(line)
			}
		}
		return fmt.Errorf("geofeed %s failed verification: %s", conf.gf, res.Failure)
	}

	if conf.db == "" {
		fmt.Printf(
			"Validated %d rows. No MMDB provided (-db), so comparison was skipped.\n",
			res.Total,
		)
		return nil
	}

	fmt.Printf(
		strings.Join(res.Diffs, "\n\n")+
			"\n\nOut of %d potential corrections, %d may be different than our current mappings\n\n",
		res.Total,
		res.Differences,
	)

	// https://stackoverflow.com/questions/18695346/how-can-i-sort-a-mapstringint-by-its-values/56706305#56706305
	asNumbers := make([]uint, 0, len(res.ASNCounts))
	for asNumber := range res.ASNCounts {
		asNumbers = append(asNumbers, asNumber)
	}
	slices.SortFunc(
		asNumbers,
		func(a, b uint) int {
			return cmp.Compare(res.ASNCounts[b], res.ASNCounts[a])
		},
	)
	for _, asNumber := range asNumbers {
		fmt.Printf("ASN: %d, count: %d\n", asNumber, res.ASNCounts[asNumber])
	}

	return nil
}

// formatInvalidRows renders one line per sample invalid row, sorted by
// invalidity type so CLI output order is deterministic. row's own
// String() already ends with the offending row's diagnostic text (and,
// for the too-few-fields case, a trailing "row: '...'"), so this does not
// quote it again -- doing so would double the quotes in that case.
func formatInvalidRows(rows map[verify.RowInvalidity]verify.InvalidRow) []string {
	lines := make([]string, 0, len(rows))
	for _, t := range slices.Sorted(maps.Keys(rows)) {
		lines = append(lines, fmt.Sprintf("%s: %s", t, rows[t]))
	}
	return lines
}

func parseFlags(program string, args []string) (c *config, output string, err error) {
	flags := flag.NewFlagSet(program, flag.ContinueOnError)
	var buf bytes.Buffer
	flags.SetOutput(&buf)

	var conf config
	flags.StringVar(&conf.gf, "gf", "", "Path to local geofeed file to verify")
	flags.StringVar(&conf.isp, "isp", "", "Path to ISP MMDB file (optional)")
	flags.StringVar(
		&conf.db,
		"db",
		"",
		"Path to MMDB file to compare the geofeed against (optional; if omitted, only the geofeed format is validated)",
	)
	displayVersion := false
	flags.BoolVar(&displayVersion, "V", false, "Display version")
	flags.BoolVar(
		&conf.laxMode,
		"lax",
		false,
		"Enable lax mode: geofeed's region code may be provided without country code prefix")
	flags.BoolVar(
		&conf.emptyOK,
		"empty-ok",
		false,
		"Allow empty geofeeds to be considered valid")

	err = flags.Parse(args)
	if err != nil {
		return nil, buf.String(), err
	}

	if displayVersion {
		log.Printf("mm-geofeed-verifier %s", version)
		//nolint:revive // preexisting
		os.Exit(0)
	}

	if conf.gf == "" {
		flags.PrintDefaults()
		return nil, buf.String(), errors.New("-gf is required")
	}

	return &conf, buf.String(), nil
}
