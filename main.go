package main

// Users will give us bulk correction files following https://tools.ietf.org/html/draft-google-self-published-geofeeds-02
// It isn't uncommon for the corrections they list to either match what we already have
// or to be worse. This script can help us work that out. Right now, it only looks
// at the ISO country code, but checking more fields should be easy enough.
import (
	"fmt"
	"github.com/oschwald/geoip2-golang"
	"github.com/yunabe/easycsv"
	"log"
	"net"
	"os"
	"strings"
)

// Usage Example:
// go run go/check_csv_corrections/main.go /path/to/corrections.csv /path/to/mmdbfile.mmdb
func main() {
	csvFilename := os.Args[1]
	mmdbFilename := os.Args[2]

	db, err := geoip2.Open(mmdbFilename)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	totalCount, correctionCount := 0, 0
	opt := easycsv.Option{
		Comment: '#',
	}
	r := easycsv.NewReaderFile(csvFilename, opt)
	err = r.Loop(func(entry *struct {
		Subnet  string `index:"0"`
		Country string `index:"1"`
		Region  string `index:"2"`
		City    string `index:"3"`
		Postal  string `index:"4"`
	}) error {
		ip, _, err := net.ParseCIDR(entry.Subnet)
		if err != nil {
			log.Fatal(err)
		}
		totalCount++
		record, err := db.City(ip)
		if err != nil {
			log.Fatal(err)
		}
		if !(strings.EqualFold(entry.Country, record.Country.IsoCode)) || !(strings.EqualFold(entry.City, record.City.Names["en"])) {
			correctionCount++
			firstSubdivision := ""
			for i, sd := range record.Subdivisions {
				if i == 0 {
					firstSubdivision = sd.IsoCode
					break
				}
			}
			fmt.Printf("Found a potential improvement: %v, current country: '%v', suggested country: '%v', current city: '%v', suggested city: '%v', current region: '%v', suggested region: '%v,'\n", entry.Subnet, record.Country.IsoCode, entry.Country, record.City.Names["en"], entry.City, firstSubdivision, entry.Region)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	fmt.Printf("\nOut of %v potential corrections, %v may be different than our current mappings\n", totalCount, correctionCount)
}
