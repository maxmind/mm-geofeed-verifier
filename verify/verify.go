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

// Result holds the outcome of verifying a geofeed file: the total number of
// rows, the number of rows that differ from the expected MMDB values,
// information about the rows that failed validation, and -- if the geofeed
// failed verification as a whole -- why.
type Result struct {
	Total       int
	Differences int
	Invalid     int
	// SampleInvalidRows holds one sample InvalidRow per RowInvalidity kind
	// encountered, keyed by kind.
	SampleInvalidRows map[RowInvalidity]InvalidRow
	// Diffs holds engineer-facing text describing each row whose MMDB
	// record differs from what the geofeed claims, one entry per row.
	Diffs []string
	// ASNCounts counts how many rows resolved to each autonomous system
	// number. It stays empty unless an ISP database was provided.
	ASNCounts map[uint]int
	// Failure says why the geofeed failed verification, or is FailureNone if
	// it passed.
	Failure FailureReason
	// FailureDiagnostic is internal, engineer-facing text about Failure --
	// where the CSV parser stopped and why, for instance. Like
	// InvalidRow.Diagnostic, it is unstable, must not be parsed, and must
	// not be shown to a geofeed's owner: Failure is what a consumer maps to
	// customer-facing wording.
	//
	// It is empty whenever the failure has no particulars to add beyond
	// Failure itself, which includes every FailureReason except
	// FailureUnreadableCSV. FailureTooManyInvalidRows describes itself
	// through SampleInvalidRows instead.
	FailureDiagnostic string
}

func newResult() Result {
	return Result{
		SampleInvalidRows: map[RowInvalidity]InvalidRow{},
		ASNCounts:         map[uint]int{},
	}
}

// FailureReason identifies why a geofeed failed verification. It is a
// verdict on the geofeed's contents rather than a fault of the verifier, so
// ProcessGeofeed reports it as a value on Result and not as an error: a
// caller that treats every non-nil error as fatal must not abort a run
// because one geofeed is malformed.
type FailureReason int

// Verification failure reasons.
const (
	// FailureNone is the zero value of FailureReason: the geofeed passed
	// verification.
	FailureNone FailureReason = iota
	// FailureNotUTF8 indicates a file encoding that is not valid UTF-8
	// (with an optional BOM). RFC 8805 says that "feeds MUST use UTF-8
	// character encoding", and nothing in the file can be read confidently
	// without it, so no rows are examined.
	FailureNotUTF8
	// FailureEmpty indicates a geofeed with no records. It is only reported
	// when Options.EmptyOK is false.
	FailureEmpty
	// FailureTooManyInvalidRows indicates incomplete compliance with the RFC
	// 8805 standards and the mode in which the verifier ran: at least one
	// row failed validation. Result.SampleInvalidRows holds one example per
	// kind of invalidity found.
	FailureTooManyInvalidRows
	// FailureUnreadableCSV indicates that the file could not be parsed as
	// CSV at all -- malformed quoting, for example. Parsing stops at the
	// offending line, so the row counts cover only the rows ahead of it.
	FailureUnreadableCSV
)

// String implements the Stringer interface.
func (fr FailureReason) String() string {
	switch fr {
	case FailureNone:
		return "FailureNone"
	case FailureNotUTF8:
		return "FailureNotUTF8"
	case FailureEmpty:
		return "FailureEmpty"
	case FailureTooManyInvalidRows:
		return "FailureTooManyInvalidRows"
	case FailureUnreadableCSV:
		return "FailureUnreadableCSV"
	default:
		return "UnknownFailureReason"
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

// ProcessGeofeed verifies the geofeed in geofeedFilename, comparing each row
// against mmdbFilename (augmented by ispFilename, if given) when an MMDB is
// provided and checking the row's format only when it is not.
//
// It returns a non-nil error only when verification could not be performed at
// all -- the file was unreadable, or an MMDB was unusable (see
// ErrDatabaseLookup). A geofeed that fails verification is not an error; it
// is a Result whose Failure says why.
func ProcessGeofeed(
	geofeedFilename,
	mmdbFilename,
	ispFilename string,
	opts Options,
) (Result, error) {
	c := newResult()

	geofeedData, err := os.ReadFile(filepath.Clean(geofeedFilename))
	if err != nil {
		if opts.HideFilePathsInErrorMessages {
			return c, fmt.Errorf("unable to open file: %w", err)
		}
		return c, fmt.Errorf("unable to open %s: %w", geofeedFilename, err)
	}

	// Strip UTF-8 BOM if present (common on files from Windows).
	geofeedData = bytes.TrimPrefix(geofeedData, []byte{0xEF, 0xBB, 0xBF})

	if !utf8.Valid(geofeedData) {
		c.Failure = FailureNotUTF8
		return c, nil
	}

	var db, ispdb *maxminddb.Reader
	if mmdbFilename != "" {
		db, err = maxminddb.Open(filepath.Clean(mmdbFilename))
		if err != nil {
			if opts.HideFilePathsInErrorMessages {
				return c, fmt.Errorf(
					"unable to open MMDB: %w: %w",
					ErrDatabaseLookup,
					err,
				)
			}
			return c, fmt.Errorf(
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
					return c, fmt.Errorf(
						"unable to open ISP MMDB: %w: %w",
						ErrDatabaseLookup,
						err,
					)
				}
				return c, fmt.Errorf(
					"unable to open ISP MMDB %s: %w: %w",
					ispFilename,
					ErrDatabaseLookup,
					err,
				)
			}
			defer ispdb.Close()
		}
	}

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
			// Malformed CSV -- unbalanced quoting, say -- is the geofeed's
			// fault, not ours, and the reader cannot resynchronize, so the
			// counts gathered so far are all there will be. The parser's own
			// message names the line and column it gave up at, which is the
			// only record of where the trouble is: no row was produced, so
			// nothing lands in SampleInvalidRows.
			c.Failure = FailureUnreadableCSV
			if opts.HideFilePathsInErrorMessages {
				c.FailureDiagnostic = fmt.Sprintf("unable to read next row: %s", err)
			} else {
				c.FailureDiagnostic = fmt.Sprintf(
					"unable to read next row in %s: %s",
					geofeedFilename,
					err,
				)
			}
			return c, nil
		}

		if isBlank(row) {
			continue
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
			c.ASNCounts,
			opts,
		)
		if err != nil {
			// A lookup failure means the database is unusable, not that
			// this row is invalid: the row was never evaluated, so it must
			// not be recorded as a sample invalid row, and processing
			// cannot usefully continue for the rows after it either.
			return c, err
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
			c.Diffs = append(c.Diffs, diffLine)
			c.Differences++
		}
	}

	if c.Total == 0 && !opts.EmptyOK {
		c.Failure = FailureEmpty
		return c, nil
	}

	if c.Invalid > 0 || len(c.SampleInvalidRows) > 0 {
		c.Failure = FailureTooManyInvalidRows
		return c, nil
	}

	return c, nil
}

// isBlank reports whether row carries no data at all. csv.Reader skips a
// genuinely empty line by itself, but a line holding only spaces or tabs
// parses as a single empty field, which would otherwise be counted as a row
// and reported as one with too few fields -- a sample row with nothing in it
// for a line the feed's author intended as blank.
func isBlank(row []string) bool {
	return len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "")
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
