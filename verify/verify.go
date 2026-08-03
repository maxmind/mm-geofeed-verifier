// Package verify provides ProcessGeofeed so that it can
// be used by other programs.
package verify

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/oschwald/maxminddb-golang/v2"
)

// CheckResult holds the total number of rows for a geofeed file,
// the number of rows that differ from expected mmdb values as well
// as information about the rows that failed validation.
// To create new CheckResult instance use NewCheckResult() func.
type CheckResult struct {
	Total       int
	Differences int
	Invalid     int
	// SampleInvalidRows holds one sample InvalidRow per RowInvalidity kind
	// encountered, keyed by kind.
	SampleInvalidRows map[RowInvalidity]InvalidRow
}

// NewCheckResult returns new CheckResult instance.
func NewCheckResult() CheckResult {
	return CheckResult{
		Total:             0,
		Differences:       0,
		Invalid:           0,
		SampleInvalidRows: map[RowInvalidity]InvalidRow{},
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
) (CheckResult, []string, map[uint]int, error) {
	c := NewCheckResult()
	var diffLines []string

	geofeedData, err := os.ReadFile(filepath.Clean(geofeedFilename))
	if err != nil {
		if opts.HideFilePathsInErrorMessages {
			return c, diffLines, nil, fmt.Errorf("unable to open file: %w", err)
		}
		return c, diffLines, nil, fmt.Errorf("unable to open %s: %w", geofeedFilename, err)
	}

	// Strip UTF-8 BOM if present (common on files from Windows).
	geofeedData = bytes.TrimPrefix(geofeedData, []byte{0xEF, 0xBB, 0xBF})

	if !utf8.Valid(geofeedData) {
		return c, diffLines, nil, ErrNotUTF8
	}

	var db, ispdb *maxminddb.Reader
	if mmdbFilename != "" {
		db, err = maxminddb.Open(filepath.Clean(mmdbFilename))
		if err != nil {
			if opts.HideFilePathsInErrorMessages {
				return c, diffLines, nil, fmt.Errorf(
					"unable to open MMDB: %w: %w",
					ErrDatabaseLookup,
					err,
				)
			}
			return c, diffLines, nil, fmt.Errorf(
				"unable to open MMDB %s: %w: %w",
				mmdbFilename,
				ErrDatabaseLookup,
				err,
			)
		}
		defer db.Close()

		if ispFilename != "" {
			ispdb, err = maxminddb.Open(filepath.Clean(ispFilename))
			if err != nil {
				if opts.HideFilePathsInErrorMessages {
					return c, diffLines, nil, fmt.Errorf(
						"unable to open ISP MMDB: %w: %w",
						ErrDatabaseLookup,
						err,
					)
				}
				return c, diffLines, nil, fmt.Errorf(
					"unable to open ISP MMDB %s: %w: %w",
					ispFilename,
					ErrDatabaseLookup,
					err,
				)
			}
			defer ispdb.Close()
		}
	}
	asnCounts := map[uint]int{}

	csvReader := csv.NewReader(bytes.NewReader(geofeedData))
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
			return c, diffLines, asnCounts, fmt.Errorf(
				"unable to read next row in %s: %w",
				geofeedFilename,
				err,
			)
		}

		c.Total++

		if len(row) < expectedFieldsPerRecord {
			if _, ok := c.SampleInvalidRows[FewerFieldsThanExpected]; !ok {
				c.SampleInvalidRows[FewerFieldsThanExpected] = newInvalidRow(
					sampleLine(csvReader, row),
					// ReuseRecord means row's backing array is overwritten by
					// the next Read; clone it so this sample survives.
					slices.Clone(row),
					fewerFieldsResult(row, expectedFieldsPerRecord),
				)
			}
			c.Invalid++
			continue
		}

		// verifyCorrection trims its correction argument in place, and that
		// argument aliases row's backing array, so the fields must be cloned
		// before the call rather than copied from row afterward.
		correction := row[:expectedFieldsPerRecord]
		fields := slices.Clone(correction)

		diffLine, result, err := verifyCorrection(
			correction,
			db,
			ispdb,
			asnCounts,
			opts,
		)
		if err != nil {
			// A lookup failure means the database is unusable, not that
			// this row is invalid: the row was never evaluated, so it must
			// not be recorded as a sample invalid row, and processing
			// cannot usefully continue for the rows after it either.
			return c, diffLines, asnCounts, err
		}
		if !result.valid {
			if _, ok := c.SampleInvalidRows[result.invalidityType]; !ok {
				c.SampleInvalidRows[result.invalidityType] = newInvalidRow(
					sampleLine(csvReader, row),
					fields,
					result,
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
		return c, diffLines, asnCounts, fmt.Errorf(
			"error while reading %s: %w",
			geofeedFilename,
			err,
		)
	}

	if c.Total == 0 && !opts.EmptyOK {
		return c, diffLines, asnCounts, ErrEmptyGeofeed
	}

	if c.Invalid > 0 || len(c.SampleInvalidRows) > 0 {
		return c, diffLines, asnCounts, ErrInvalidGeofeed
	}

	return c, diffLines, asnCounts, nil
}

// sampleLine returns the geofeed file line of row via csvReader's FieldPos.
// FieldPos panics if the field index is out of range; csv.Reader never
// actually returns a zero-field record with a nil error, but the guard is
// cheap insurance against a panic if that ever changes.
func sampleLine(csvReader *csv.Reader, row []string) int {
	if len(row) == 0 {
		return 0
	}
	line, _ := csvReader.FieldPos(0)
	return line
}

type verificationResult struct {
	valid          bool
	invalidityType RowInvalidity
	// diagnostic is internal, engineer-facing text: it feeds
	// InvalidRow.Diagnostic (rendered as "line %d: %s" by InvalidRow.String)
	// and may embed raw error text or the row itself.
	diagnostic string
	// reason is customer-facing wording for the same failure: it feeds
	// InvalidRow.Reason instead of diagnostic. It must be a fixed string
	// per invalidity type -- never interpolating a row field or a value
	// derived from one -- and must never surface an internal error or a
	// library name.
	reason string
}

// newInvalidRow builds the InvalidRow for a sample found on the given file
// line, with the given fields, from a verificationResult describing why
// it's invalid.
func newInvalidRow(line int, fields []string, res verificationResult) InvalidRow {
	return InvalidRow{
		Line:       line,
		Type:       res.invalidityType,
		Fields:     fields,
		Reason:     res.reason,
		Diagnostic: res.diagnostic,
	}
}

// fewerFieldsResult builds the verificationResult for a row with fewer
// fields than expected.
func fewerFieldsResult(row []string, expected int) verificationResult {
	return verificationResult{
		valid:          false,
		invalidityType: FewerFieldsThanExpected,
		diagnostic: fmt.Sprintf(
			"expected %d fields but got %d, row: '%s'",
			expected,
			len(row),
			strings.Join(row, ","),
		),
		reason: fmt.Sprintf(
			"The row has %d fields, but a geofeed row requires %d.",
			len(row),
			expected,
		),
	}
}

func invalidRegionCodeResult(correction []string) verificationResult {
	return verificationResult{
		valid:          false,
		invalidityType: InvalidRegionCode,
		diagnostic: fmt.Sprintf(
			"invalid ISO 3166-2 region code format in strict (default) mode, row: '%s'",
			strings.Join(correction, ","),
		),
		reason: "The region code is not in ISO 3166-2 format (for example, US-CA). " +
			"Enable lax mode to accept a region code without the country prefix.",
	}
}

func verifyCorrection(
	correction []string,
	db, ispdb *maxminddb.Reader,
	asnCounts map[uint]int,
	opts Options,
) (string, verificationResult, error) {
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
			valid:          false,
			invalidityType: EmptyNetwork,
			diagnostic: fmt.Sprintf(
				"network field is empty, row: '%s'",
				strings.Join(correction, ","),
			),
			reason: "The network field is empty.",
		}, nil
	}
	if !(strings.Contains(networkOrIP, "/")) {
		if strings.Contains(networkOrIP, ":") {
			networkOrIP += "/64"
		} else {
			networkOrIP += "/32"
		}
	}
	network, err := netip.ParsePrefix(networkOrIP)
	if err != nil {
		return "", verificationResult{
			valid:          false,
			invalidityType: UnableToParseNetwork,
			diagnostic:     fmt.Sprintf("unable to parse network %s: %s", networkOrIP, err),
			reason:         "The network field is not a valid IP address or CIDR.",
		}, nil
	}

	if db == nil {
		// format-only mode: only the DB-independent region-code format rule applies.
		if !strings.Contains(correction[2], "-") && correction[2] != "" && !opts.LaxMode {
			return "", invalidRegionCodeResult(correction), nil
		}
		return "", verificationResult{
			valid:          true,
			invalidityType: UnknownInvalidity,
			diagnostic:     "",
		}, nil
	}

	// XXX - should we be checking the whole network?
	result := db.Lookup(network.Addr())

	// A record present for this network always decodes cleanly (a missing
	// record yields a nil error and a zero-valued struct, per
	// Result.Decode), so an error here means the database itself is
	// unusable, not that the row is invalid -- that's why this returns an
	// error instead of a verificationResult.
	var cityRecord struct {
		Subdivisions []struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"subdivisions"`
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
		City struct {
			Names struct {
				English string `maxminddb:"en"`
			} `maxminddb:"names"`
		} `maxminddb:"city"`
		Postal struct {
			Code string `maxminddb:"code"`
		} `maxminddb:"postal"`
	}
	if err := result.Decode(&cityRecord); err != nil {
		return "", verificationResult{}, fmt.Errorf(
			"decoding city database record for %s: %w: %w",
			networkOrIP,
			ErrDatabaseLookup,
			err,
		)
	}
	countryCode := cityRecord.Country.ISOCode
	cityName := cityRecord.City.Names.English
	postalCode := cityRecord.Postal.Code

	// Subdivisions run least to most specific, so the last entry is the
	// most specific one. A record with no subdivisions leaves this empty.
	mostSpecificSubdivision := ""
	if n := len(cityRecord.Subdivisions); n > 0 {
		mostSpecificSubdivision = cityRecord.Subdivisions[n-1].ISOCode
	}

	// ISO-3166-2 region codes are prefixed with the ISO country code,
	// in strict (default) mode we require this format.
	// In "--lax" mode both region code formats (with or without country code) are accepted.
	if strings.Contains(correction[2], "-") {
		mostSpecificSubdivision = countryCode + "-" + mostSpecificSubdivision
	} else if correction[2] != "" && !opts.LaxMode {
		return "", invalidRegionCodeResult(correction), nil
	}

	asNumber := uint(0)
	asName := ""
	ispName := ""
	if ispdb != nil {
		var ispRecord struct {
			AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
			AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
			ISP                          string `maxminddb:"isp"`
		}
		// XXX - should we be checking the whole network?
		// A missing record decodes cleanly to ispRecord's zero value, as
		// with the city record above; an error here means the ISP
		// database itself is unusable.
		if err := ispdb.Lookup(network.Addr()).Decode(&ispRecord); err != nil {
			return "", verificationResult{}, fmt.Errorf(
				"decoding ISP database record for %s: %w: %w",
				networkOrIP,
				ErrDatabaseLookup,
				err,
			)
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

	if !(strings.EqualFold(correction[1], countryCode)) {
		foundDiff = true
		lines = append(
			lines,
			fmt.Sprintf(
				"current country: '%s'%ssuggested country: '%s'",
				countryCode,
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

	if !(strings.EqualFold(correction[3], cityName)) {
		foundDiff = true
		lines = append(
			lines,
			fmt.Sprintf(
				"current city: '%s'%ssuggested city: '%s'",
				cityName,
				indent,
				correction[3],
			),
		)
	}

	// if no postal code is provided in the correction, do not report on any
	// differences; postal codes are frequently omitted, and as of 2020-08-01
	// the postal code field is considered deprecated in RFC 8805
	if correction[4] != "" && !(strings.EqualFold(correction[4], postalCode)) {
		foundDiff = true
		lines = append(
			lines,
			fmt.Sprintf(
				"current postal code: '%s'%ssuggested postal code: '%s'",
				postalCode,
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
			valid:          true,
			invalidityType: UnknownInvalidity,
			diagnostic:     "",
		}, nil
	}
	return "", verificationResult{
		valid:          true,
		invalidityType: UnknownInvalidity,
		diagnostic:     "",
	}, nil
}
