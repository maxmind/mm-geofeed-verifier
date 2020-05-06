package main

// Users will give us bulk correction files following https://tools.ietf.org/html/draft-google-self-published-geofeeds-02
// It isn't uncommon for the corrections they list to either match what we already have
// or to be worse. This script can help us work that out. Right now, it only looks
// at the ISO country code, but checking more fields should be easy enough.
import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	geoip2 "github.com/oschwald/geoip2-golang"
)

// Usage Example:
// go run go/check_csv_corrections/main.go /path/to/corrections.csv /path/to/mmdbfile.mmdb
func main() {
	geofeedFilename, mmdbFilename, err := parseArgs()
	if err != nil {
		log.Fatal(err)
	}

	db, err := geoip2.Open(mmdbFilename)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Println(err)
		}
	}()

	totalCount, correctionCount := 0, 0
	geofeedFH, err := os.Open(geofeedFilename) //nolint: gosec // linter doesn't realize we are cleaning the filepath
	if err != nil {
		log.Fatal(err)
	}
	csvReader := csv.NewReader(geofeedFH)
	csvReader.ReuseRecord = true
	csvReader.Comment = '#'
	csvReader.FieldsPerRecord = 5
	csvReader.TrimLeadingSpace = true
	defer func() {
		if err := geofeedFH.Close(); err != nil {
			log.Println(err)
		}
	}()

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			geofeedFH.Close() //nolint: gosec, errcheck
			log.Fatal(err)
		}
		totalCount++
		currentCorrectionCount, err := verifyCorrection(row, db)
		if err != nil {
			geofeedFH.Close() //nolint: gosec, errcheck
			log.Fatal(err)
		}
		correctionCount += currentCorrectionCount
	}
	if err != nil && err != io.EOF {
		log.Fatalf("Failed to read file: %s", err)
	}
	fmt.Printf(
		"\nOut of %d potential corrections, %d may be different than our current mappings\n",
		totalCount,
		correctionCount,
	)
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
			networkOrIP += "/128"
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
	// ISO-3166-2 region codes should be prefixed with the ISO country code,
	// but we accept just the region code part
	if strings.Contains(correction[2], "-") {
		firstSubdivision = mmdbRecord.Country.IsoCode + "-" + firstSubdivision
	}
	if !(strings.EqualFold(correction[1], mmdbRecord.Country.IsoCode)) ||
		!(strings.EqualFold(correction[2], firstSubdivision)) ||
		!(strings.EqualFold(correction[3], mmdbRecord.City.Names["en"])) {
		fmt.Printf(
			"Found a potential improvement: %s, current country: '%s',suggested country: '%s', current city: '%s', suggested city: '%s', current region: '%s', suggested region: '%s'\n",
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

func parseArgs() (string, string, error) {
	geofeedPath := flag.String(
		"geofeed-path",
		"",
		"Path to the local geofeed file to verify",
	)

	mmdbPath := flag.String(
		"mmdb-path",
		"/usr/local/share/GeoIP/GeoIP2-City.mmdb",
		"Path to MMDB file to compare geofeed file against",
	)
	flag.Parse()

	cleanGeofeedPath := filepath.Clean(*geofeedPath)
	cleanMMDBPath := filepath.Clean(*mmdbPath)

	var err error
	if cleanGeofeedPath == "." { // result of empty string, probably no arg given
		//err = errors.New("'--geofeed-path' is required, and should be a path to a local geofeed file")
		err = fmt.Errorf("'--geofeed-path' is required")
	}
	return cleanGeofeedPath, cleanMMDBPath, err

}
