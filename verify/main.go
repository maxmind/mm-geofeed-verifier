// Package verify provides ProcessGeofeed so that it can
// be used by other programs.
package verify

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"path/filepath"
	"strings"

	"github.com/TomOnTime/utfutil"

	geoip2 "github.com/oschwald/geoip2-golang"
)

// Counts holds the total number of rows for a geofeed file
// as well as the number of rows that differ from expected mmdb values.
type Counts struct {
	Total       int
	Differences int
}

// ProcessGeofeed attempts to validate a given geofeedFilename.
func ProcessGeofeed(geofeedFilename, mmdbFilename, ispFilename string) (Counts, []string, map[uint]int, error) {
	var c Counts
	var diffLines []string
	geofeedFH, err := utfutil.OpenFile(filepath.Clean(geofeedFilename), utfutil.UTF8)
	if err != nil {
		return c, diffLines, nil, fmt.Errorf("unable to open %s: %w", geofeedFilename, err)
	}
	defer func() {
		if err := geofeedFH.Close(); err != nil {
			log.Println(err)
		}
	}()

	db, err := geoip2.Open(filepath.Clean(mmdbFilename))
	if err != nil {
		return c, diffLines, nil, fmt.Errorf("unable to open MMDB %s: %w", mmdbFilename, err)
	}
	defer db.Close()

	var ispdb *geoip2.Reader
	if len(ispFilename) > 0 {
		ispdb, err = geoip2.Open(filepath.Clean(ispFilename))
		if err != nil {
			return c, diffLines, nil, fmt.Errorf("unable to open MMDB %s: %w", ispFilename, err)
		}
		defer ispdb.Close()
	}
	asnCounts := make(map[uint]int)

	csvReader := csv.NewReader(geofeedFH)
	csvReader.ReuseRecord = true
	csvReader.Comment = '#'
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true

	const expectedFieldsPerRecord = 5

	rowCount := 0

	for {
		row, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		rowCount++
		if err != nil {
			return c, diffLines, asnCounts,
				fmt.Errorf("unable to read next row in %s: %w", geofeedFilename, err)
		}
		if len(row) < expectedFieldsPerRecord {
			return c, nil, nil, fmt.Errorf(
				"saw fewer than the expected %d fields at line %d: '%s'",
				expectedFieldsPerRecord,
				rowCount,
				strings.Join(row, ","),
			)
		}

		c.Total++
		diffLine, err := verifyCorrection(row[:expectedFieldsPerRecord], db, ispdb, asnCounts)
		if err != nil {
			return c, diffLines, asnCounts, err
		}

		if len(diffLine) > 0 {
			diffLines = append(diffLines, diffLine)
			c.Differences++
		}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return c, diffLines, asnCounts,
			fmt.Errorf("error while reading %s: %w", geofeedFilename, err)
	}
	return c, diffLines, asnCounts, nil
}

func verifyCorrection(correction []string, db, ispdb *geoip2.Reader, asnCounts map[uint]int) (string, error) {
	/*
	   0: network (CIDR or single IP)
	   1: ISO-3166 country code
	   2: ISO-3166-2 region code
	   3: city name
	   4: postal code
	*/

	for i, v := range correction {
		correction[i] = strings.TrimSpace(v)
	}

	networkOrIP := correction[0]
	if networkOrIP == "" {
		return "", errors.New("network field is empty")
	}
	if !(strings.Contains(networkOrIP, "/")) {
		if strings.Contains(networkOrIP, ":") {
			networkOrIP += "/64"
		} else {
			networkOrIP += "/32"
		}
	}
	network, _, err := net.ParseCIDR(networkOrIP) //nolint:forbidigo // use of net is ok for now
	if err != nil {
		return "", fmt.Errorf("unable to parse network %s: %w", networkOrIP, err)
	}

	mmdbRecord, err := db.City(network)
	if err != nil {
		return "", fmt.Errorf("unable to find city record for %s: %w", networkOrIP, err)
	}

	lastSubdivision := ""
	if len(mmdbRecord.Subdivisions) > 0 {
		lastSubdivision = mmdbRecord.Subdivisions[len(mmdbRecord.Subdivisions)-1].IsoCode
	}
	// ISO-3166-2 region codes are prefixed with the ISO country code,
	// but we accept just the region code part
	if strings.Contains(correction[2], "-") {
		lastSubdivision = mmdbRecord.Country.IsoCode + "-" + lastSubdivision
	}

	asNumber := uint(0)
	asName := ""
	ispName := ""
	if ispdb != nil {
		ispRecord, err := ispdb.ISP(network)
		if err != nil {
			return "", fmt.Errorf("unable to find ISP record for %s: %w", networkOrIP, err)
		}
		asNumber = ispRecord.AutonomousSystemNumber
		asName = ispRecord.AutonomousSystemOrganization
		ispName = ispRecord.ISP
	}
	if asNumber > 0 {
		asnCounts[asNumber]++
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

	if !(strings.EqualFold(correction[2], lastSubdivision)) {
		foundDiff = true
		lines = append(
			lines,
			fmt.Sprintf(
				"current region: '%s'%ssuggested region: '%s'",
				lastSubdivision,
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
	// the postal code field is considered deprecated in RFC 8805
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
		if asNumber > 0 {
			lines = append(
				lines,
				fmt.Sprintf(
					"AS Number: %d",
					asNumber,
				),
			)
		}
		if asName != "" {
			lines = append(
				lines,
				fmt.Sprintf(
					"AS Name: %s",
					asName,
				),
			)
		}
		if ispName != "" {
			lines = append(
				lines,
				fmt.Sprintf(
					"ISP Name: %s",
					ispName,
				),
			)
		}

		return strings.Join(lines, "\n"+indent), nil
	}
	return "", nil
}
