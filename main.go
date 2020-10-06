package main

/*
This script is meant to help verify 'bulk correction' files for submission
to MaxMind. The files are expected to (mostly) follow the format provided by the RFC at
https://tools.ietf.org/html/draft-google-self-published-geofeeds-09
Region codes without the country prefix are accepted. eg, 'NY' is allowed, along with
'US-NY' for the state of New York in the United States.
Beyond verifying that the format of the data is correct, the script will also compare
the corrections against a given MMDB, reporting on how many corrections differ from
the contents in the database.
*/

import (
	"bytes"
	"encoding/csv"
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
	conf, output, err := parseFlags(os.Args[0], os.Args[1:])
	if err == flag.ErrHelp {
		fmt.Println(output)
		os.Exit(2)
	} else if err != nil {
		fmt.Println(output)
		log.Fatal(err)
	}

	c, err := processGeofeed(conf.gf, conf.db)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"\nOut of %d potential corrections, %d may be different than our current mappings\n",
		c.total,
		c.differences,
	)
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
	return &conf, buf.String(), nil
}

func processGeofeed(geofeedFilename, mmdbFilename string) (counts, error) {
	var c counts
	geofeedFH, err := utfutil.OpenFile(filepath.Clean(geofeedFilename), utfutil.UTF8)
	if err != nil {
		return c, err
	}
	defer func() {
		if err := geofeedFH.Close(); err != nil {
			log.Println(err)
		}
	}()

	db, err := geoip2.Open(filepath.Clean(mmdbFilename))
	if err != nil {
		return c, err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Println(err)
		}
	}()

	csvReader := csv.NewReader(geofeedFH)
	csvReader.ReuseRecord = true
	csvReader.Comment = '#'
	csvReader.FieldsPerRecord = 5
	csvReader.TrimLeadingSpace = true

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			geofeedFH.Close() //nolint: gosec, errcheck
			return c, err
		}
		c.total++
		currentCorrectionCount, err := verifyCorrection(row, db)
		if err != nil {
			geofeedFH.Close() //nolint: gosec, errcheck
			return c, err
		}
		c.differences += currentCorrectionCount
	}
	if err != nil && err != io.EOF {
		return c, err
	}
	return c, nil
}

func verifyCorrection(correction []string, db *geoip2.Reader) (int, error) {
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
		return 0, err
	}
	mmdbRecord, err := db.City(network)
	if err != nil {
		return 0, err
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
	if !(strings.EqualFold(correction[1], mmdbRecord.Country.IsoCode)) ||
		!(strings.EqualFold(correction[2], firstSubdivision)) ||
		!(strings.EqualFold(correction[3], mmdbRecord.City.Names["en"])) {
		diffLine := "Found a potential improvement: '%s'\n" +
			"\t\tcurrent country: '%s'\t\tsuggested country: '%s'\n" +
			"\t\tcurrent city: '%s'\t\tsuggested city: '%s'\n" +
			"\t\tcurrent region: '%s'\t\tsuggested region: '%s'\n\n"
		fmt.Printf(
			diffLine,
			networkOrIP,
			mmdbRecord.Country.IsoCode,
			correction[1],
			mmdbRecord.City.Names["en"],
			correction[3],
			firstSubdivision,
			correction[2],
		)
		return 1, nil
	}
	return 0, nil
}
