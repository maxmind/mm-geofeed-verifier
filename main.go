package main

/*
This script is meant to help verify 'bulk correction' files for submission
to MaxMind. The files are expected to (mostly) follow the format provided by the RFC at
https://datatracker.ietf.org/doc/rfc8805/
Region codes without the country prefix are accepted. eg, 'NY' is allowed, along with
'US-NY' for the state of New York in the United States.
Beyond verifying that the format of the data is correct, the script will also compare
the corrections against a given MMDB, reporting on how many corrections differ from
the contents in the database.
*/

import (
	"bytes"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/TomOnTime/utfutil"

	geoip2 "github.com/oschwald/geoip2-golang"
)

type config struct {
	gf string
	db string
}

type counts struct {
	total       int
	differences int
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

	c, diffLines, err := processGeofeed(conf.gf, conf.db)
	if err != nil {
		return err
	}

	fmt.Printf(
		strings.Join(diffLines, "\n\n")+
			"\n\nOut of %d potential corrections, %d may be different than our current mappings\n\n",
		c.total,
		c.differences,
	)
	return nil
}

func parseFlags(program string, args []string) (c *config, output string, err error) {
	flags := flag.NewFlagSet(program, flag.ContinueOnError)
	var buf bytes.Buffer
	flags.SetOutput(&buf)

	var conf config
	flags.StringVar(&conf.gf, "gf", "", "Path to local geofeed file to verify")
	flags.StringVar(
		&conf.db,
		"db",
		"/usr/local/share/GeoIP/GeoIP2-City.mmdb",
		"Path to MMDB file to compare geofeed file against",
	)

	err = flags.Parse(args)
	if err != nil {
		return nil, buf.String(), err
	}

	if conf.gf == "" && conf.db == "" {
		flags.PrintDefaults()
		return nil, buf.String(), errors.New(
			"-gf is required and -db can not be an emptry string",
		)
	}
	if conf.gf == "" {
		flags.PrintDefaults()
		return nil, buf.String(), errors.New("-gf is required")
	}
	if conf.db == "" {
		flags.PrintDefaults()
		return nil, buf.String(), errors.New("-db is required")
	}

	return &conf, buf.String(), nil
}

func processGeofeed(geofeedFilename, mmdbFilename string) (counts, []string, error) {
	var c counts
	var diffLines []string
	geofeedFH, err := utfutil.OpenFile(filepath.Clean(geofeedFilename), utfutil.UTF8)
	if err != nil {
		return c, diffLines, err
	}
	defer func() {
		if err := geofeedFH.Close(); err != nil {
			log.Println(err)
		}
	}()

	db, err := geoip2.Open(filepath.Clean(mmdbFilename))
	if err != nil {
		return c, diffLines, err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Println(err)
		}
	}()

	csvReader := csv.NewReader(geofeedFH)
	csvReader.ReuseRecord = true
	csvReader.Comment = '#'
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true

	const expectedFieldsPerRecord = 5

	rowCount := 0

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		rowCount++
		if err != nil {
			return c, diffLines, err
		}
		if len(row) < expectedFieldsPerRecord {
			return c, nil, fmt.Errorf(
				"saw fewer than the expected %d fields at line %d",
				expectedFieldsPerRecord,
				rowCount,
			)
		}

		c.total++
		currentCorrectionCount, diffLine, err := verifyCorrection(row[:expectedFieldsPerRecord], db)
		diffLines = append(diffLines, diffLine)
		if err != nil {
			return c, diffLines, err
		}

		c.differences += currentCorrectionCount
	}
	if err != nil && err != io.EOF {
		return c, diffLines, err
	}
	return c, diffLines, nil
}

func verifyCorrection(correction []string, db *geoip2.Reader) (int, string, error) {
	/*
	   0: network (CIDR or single IP)
	   1: ISO-3166 country code
	   2: ISO-3166-2 region code
	   3: city name
	   4: postal code
	*/

	networkOrIP := correction[0]
	if !(strings.Contains(networkOrIP, "/")) {
		if strings.Contains(networkOrIP, ":") {
			networkOrIP += "/64"
		} else {
			networkOrIP += "/32"
		}
	}
	network, _, err := net.ParseCIDR(networkOrIP)
	if err != nil {
		return 0, "", err
	}

	mmdbRecord, err := db.City(network)
	if err != nil {
		return 0, "", err
	}

	firstSubdivision := ""
	if len(mmdbRecord.Subdivisions) > 0 {
		firstSubdivision = mmdbRecord.Subdivisions[0].IsoCode
	}
	// ISO-3166-2 region codes are prefixed with the ISO country code,
	// but we accept just the region code part
	if strings.Contains(correction[2], "-") {
		firstSubdivision = mmdbRecord.Country.IsoCode + "-" + firstSubdivision
	}

	const indent = "\t\t"

	foundDiff := false
	lines := []string{fmt.Sprintf("\nFound a potential improvement: '%s'", networkOrIP)}

	if !(strings.EqualFold(correction[1], mmdbRecord.Country.IsoCode)) {
		foundDiff = true
		lines = append(
			lines,
			fmt.Sprintf(
				"current country: '%s'%ssuggested country: '%s'",
				mmdbRecord.Country.IsoCode,
				indent,
				correction[1],
			),
		)
	}

	if !(strings.EqualFold(correction[2], firstSubdivision)) {
		foundDiff = true
		lines = append(
			lines,
			fmt.Sprintf(
				"current region: '%s'%ssuggested region: '%s'",
				firstSubdivision,
				indent,
				correction[2],
			),
		)
	}

	if !(strings.EqualFold(correction[3], mmdbRecord.City.Names["en"])) {
		foundDiff = true
		lines = append(
			lines,
			fmt.Sprintf(
				"current city: '%s'%ssuggested city: '%s'",
				mmdbRecord.City.Names["en"],
				indent,
				correction[3],
			),
		)
	}

	// if no postal code is provided in the correction, do not report on any
	// differences; postal codes are frequently omitted, and as of 2020-08-01 are
	// the postal code field is considered deprecated
	if correction[4] != "" && !(strings.EqualFold(correction[4], mmdbRecord.Postal.Code)) {
		foundDiff = true
		lines = append(
			lines,
			fmt.Sprintf(
				"current postal code: '%s'%ssuggested postal code: '%s'",
				mmdbRecord.Postal.Code,
				indent,
				correction[4],
			),
		)

	}

	if foundDiff {
		return 1, strings.Join(lines, "\n"+indent), nil
	}
	return 0, "", nil
}
