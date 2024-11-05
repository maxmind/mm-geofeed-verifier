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

// CheckResult holds the total number of rows for a geofeed file,
// the number of rows that differ from expected mmdb values as well
// as information about the rows that failed validation.
// To create new CheckResult instance use NewCheckResult() func.
type CheckResult struct {
	Total             int
	Differences       int
	Invalid           int
	SampleInvalidRows map[RowInvalidity]string
}

// NewCheckResult returns new CheckResult instance.
func NewCheckResult() CheckResult {
	return CheckResult{
		Total:             0,
		Differences:       0,
		Invalid:           0,
		SampleInvalidRows: map[RowInvalidity]string{},
	}
}

// Options contains configuration options for geofeed verification.
type Options struct {
	// // LaxMode controls validation for region codes. If LaxMode is false
	// (default), ISO-3166-2 region codes format is required. Otherwise region
	// code is accepted both with or without country code.
	LaxMode bool
	// HideFilePathsInErrorMessages, if set to true, will prevent file paths
	// from appearing in error messages. This reduces information leakage in
	// contexts where the error messages might be shared.
	HideFilePathsInErrorMessages bool
	// EmptyOK, if set to true, will consider a geofeed with no records to be
	// valid. The default behavior (false) requires a geofeed to not be empty.
	EmptyOK bool
}

// ProcessGeofeed attempts to validate a given geofeedFilename.
func ProcessGeofeed(
	geofeedFilename,
	mmdbFilename,
	ispFilename string,
	opts Options,
) (CheckResult, []string, map[uint]int, error) { //nolint:unparam // false positive on map[uint]int
	c := NewCheckResult()
	var diffLines []string

	// Use utfutil to remove a BOM, if present (common on files from Windows).
	// See https://github.com/golang/go/issues/33887.
	geofeedFH, err := utfutil.OpenFile(filepath.Clean(geofeedFilename), utfutil.UTF8)
	if err != nil {
		if opts.HideFilePathsInErrorMessages {
			return c, diffLines, nil, fmt.Errorf("unable to open file: %w", err)
		}
		return c, diffLines, nil, fmt.Errorf("unable to open %s: %w", geofeedFilename, err)
	}
	defer func() {
		if err := geofeedFH.Close(); err != nil {
			log.Println(err)
		}
	}()

	db, err := geoip2.Open(filepath.Clean(mmdbFilename))
	if err != nil {
		if opts.HideFilePathsInErrorMessages {
			return c, diffLines, nil, fmt.Errorf("unable to open MMDB: %w", err)
		}
		return c, diffLines, nil, fmt.Errorf("unable to open MMDB %s: %w", mmdbFilename, err)
	}
	defer db.Close()

	var ispdb *geoip2.Reader
	if ispFilename != "" {
		ispdb, err = geoip2.Open(filepath.Clean(ispFilename))
		if err != nil {
			if opts.HideFilePathsInErrorMessages {
				return c, diffLines, nil, fmt.Errorf("unable to open ISP MMDB: %w", err)
			}
			return c, diffLines, nil, fmt.Errorf("unable to open ISP MMDB %s: %w", ispFilename, err)
		}
		defer ispdb.Close()
	}
	asnCounts := map[uint]int{}

	csvReader := csv.NewReader(geofeedFH)
	csvReader.ReuseRecord = true
	csvReader.Comment = '#'
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true

	const expectedFieldsPerRecord = 5

	for {
		row, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if opts.HideFilePathsInErrorMessages {
				return c, diffLines, asnCounts, fmt.Errorf("unable to read next row: %w", err)
			}
			return c, diffLines, asnCounts, fmt.Errorf("unable to read next row in %s: %w", geofeedFilename, err)
		}

		c.Total++

		if len(row) < expectedFieldsPerRecord {
			if _, ok := c.SampleInvalidRows[FewerFieldsThanExpected]; !ok {
				c.SampleInvalidRows[FewerFieldsThanExpected] = fmt.Sprintf(
					"line %d: expected %d fields but got %d, row: '%s'",
					c.Total,
					expectedFieldsPerRecord,
					len(row),
					strings.Join(row, ","),
				)
			}
			c.Invalid++
			continue
		}

		diffLine, result := verifyCorrection(row[:expectedFieldsPerRecord], db, ispdb, asnCounts, opts)
		if !result.valid {
			if _, ok := c.SampleInvalidRows[result.invalidityType]; !ok {
				c.SampleInvalidRows[result.invalidityType] = fmt.Sprintf(
					"line %d: %s",
					c.Total,
					result.invalidityReason,
				)
			}
			c.Invalid++
			continue
		}

		if diffLine != "" {
			diffLines = append(diffLines, diffLine)
			c.Differences++
		}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		if opts.HideFilePathsInErrorMessages {
			return c, diffLines, asnCounts, fmt.Errorf("error reading file: %w", err)
		}
		return c, diffLines, asnCounts, fmt.Errorf("error while reading %s: %w", geofeedFilename, err)
	}

	if c.Total == 0 && !opts.EmptyOK {
		return c, diffLines, asnCounts, ErrEmptyGeofeed
	}

	if c.Invalid > 0 || len(c.SampleInvalidRows) > 0 {
		return c, diffLines, asnCounts, ErrInvalidGeofeed
	}

	return c, diffLines, asnCounts, nil
}

type verificationResult struct {
	valid            bool
	invalidityType   RowInvalidity
	invalidityReason string
}

func verifyCorrection(
	correction []string,
	db, ispdb *geoip2.Reader,
	asnCounts map[uint]int,
	opts Options,
) (string, verificationResult) {
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
		return "", verificationResult{
			valid:            false,
			invalidityType:   EmptyNetwork,
			invalidityReason: fmt.Sprintf("network field is empty, row: '%s'", strings.Join(correction, ",")),
		}
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
		return "", verificationResult{
			valid:            false,
			invalidityType:   UnableToParseNetwork,
			invalidityReason: fmt.Sprintf("unable to parse network %s: %s", networkOrIP, err),
		}
	}

	mmdbRecord, err := db.City(network)
	if err != nil {
		return "", verificationResult{
			valid:            false,
			invalidityType:   UnableToFindCityRecord,
			invalidityReason: fmt.Sprintf("unable to find city record for %s: %s", networkOrIP, err),
		}
	}

	mostSpecificSubdivision := ""
	if len(mmdbRecord.Subdivisions) > 0 {
		mostSpecificSubdivision = mmdbRecord.Subdivisions[len(mmdbRecord.Subdivisions)-1].IsoCode
	}
	// ISO-3166-2 region codes are prefixed with the ISO country code,
	// in strict (default) mode we require this format.
	// In "--lax" mode both region code formats (with or without country code) are accepted.
	if strings.Contains(correction[2], "-") {
		mostSpecificSubdivision = mmdbRecord.Country.IsoCode + "-" + mostSpecificSubdivision
	} else if correction[2] != "" && !opts.LaxMode {
		return "", verificationResult{
			valid:          false,
			invalidityType: InvalidRegionCode,
			invalidityReason: fmt.Sprintf(
				"invalid ISO 3166-2 region code format in strict (default) mode, row: '%s'",
				strings.Join(correction, ","),
			),
		}
	}

	asNumber := uint(0)
	asName := ""
	ispName := ""
	if ispdb != nil {
		ispRecord, err := ispdb.ISP(network)
		if err != nil {
			return "", verificationResult{
				valid:            false,
				invalidityType:   UnableToFindISPRecord,
				invalidityReason: fmt.Sprintf("unable to find ISP record for %s: %s", networkOrIP, err),
			}
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

	if !(strings.EqualFold(correction[2], mostSpecificSubdivision)) {
		foundDiff = true
		lines = append(
			lines,
			fmt.Sprintf(
				"current region: '%s'%ssuggested region: '%s'",
				mostSpecificSubdivision,
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
				"AS Name: "+asName,
			)
		}
		if ispName != "" {
			lines = append(
				lines,
				"ISP Name: "+ispName,
			)
		}

		return strings.Join(lines, "\n"+indent), verificationResult{
			valid:            true,
			invalidityType:   RowInvalidity(-1),
			invalidityReason: "",
		}
	}
	return "", verificationResult{
		valid:            true,
		invalidityType:   RowInvalidity(-1),
		invalidityReason: "",
	}
}
