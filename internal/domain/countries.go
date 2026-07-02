package domain

import (
	"strings"
)

// CountryCode holds ISO codes for a country.
type CountryCode struct {
	Alpha2 string
	Alpha3 string
}

// CountryMap provides a mapping from common names/ISO codes to ISO Alpha-2 and Alpha-3.
var CountryMap = map[string]CountryCode{
	"UNITED STATES":    {Alpha2: "US", Alpha3: "USA"},
	"UNITED STATES OF AMERICA": {Alpha2: "US", Alpha3: "USA"},
	"USA":              {Alpha2: "US", Alpha3: "USA"},
	"US":               {Alpha2: "US", Alpha3: "USA"},
	"ETHIOPIA":         {Alpha2: "ET", Alpha3: "ETH"},
	"ETH":              {Alpha2: "ET", Alpha3: "ETH"},
	"ET":               {Alpha2: "ET", Alpha3: "ETH"},
	"UNITED KINGDOM":   {Alpha2: "GB", Alpha3: "GBR"},
	"GREAT BRITAIN":    {Alpha2: "GB", Alpha3: "GBR"},
	"UK":               {Alpha2: "GB", Alpha3: "GBR"},
	"GBR":              {Alpha2: "GB", Alpha3: "GBR"},
	"GB":               {Alpha2: "GB", Alpha3: "GBR"},
	"CANADA":           {Alpha2: "CA", Alpha3: "CAN"},
	"CAN":              {Alpha2: "CA", Alpha3: "CAN"},
	"CA":               {Alpha2: "CA", Alpha3: "CAN"},
	"GERMANY":          {Alpha2: "DE", Alpha3: "DEU"},
	"DEU":              {Alpha2: "DE", Alpha3: "DEU"},
	"DE":               {Alpha2: "DE", Alpha3: "DEU"},
	"FRANCE":           {Alpha2: "FR", Alpha3: "FRA"},
	"FRA":              {Alpha2: "FR", Alpha3: "FRA"},
	"FR":               {Alpha2: "FR", Alpha3: "FRA"},
	"ITALY":            {Alpha2: "IT", Alpha3: "ITA"},
	"ITA":              {Alpha2: "IT", Alpha3: "ITA"},
	"IT":               {Alpha2: "IT", Alpha3: "ITA"},
	"POLAND":           {Alpha2: "PL", Alpha3: "POL"},
	"PL":               {Alpha2: "PL", Alpha3: "POL"},
	"POL":              {Alpha2: "PL", Alpha3: "POL"},
	"MALAYSIA":         {Alpha2: "MY", Alpha3: "MYS"},
	"MYS":              {Alpha2: "MY", Alpha3: "MYS"},
	"MY":               {Alpha2: "MY", Alpha3: "MYS"},
}

// GetCountryCodes resolves a string (name or code) to its ISO Alpha-2 and Alpha-3 codes.
func GetCountryCodes(input string) (alpha2, alpha3 string) {
	upper := strings.ToUpper(strings.TrimSpace(input))
	if codes, ok := CountryMap[upper]; ok {
		return codes.Alpha2, codes.Alpha3
	}
	// Fallback logic
	if len(upper) == 2 {
		return upper, ""
	}
	if len(upper) == 3 {
		return "", upper
	}
	return upper, "" // Return as is if unknown but non-empty
}

// StateMap provides mapping for common state names to 2-letter codes (US only for now).
var StateMap = map[string]string{
	"NEW YORK":      "NY",
	"CALIFORNIA":    "CA",
	"TEXAS":         "TX",
	"FLORIDA":       "FL",
	"ILLINOIS":      "IL",
	"PENNSYLVANIA": "PA",
	"OHIO":          "OH",
	"GEORGIA":       "GA",
	"WASHINGTON":    "WA",
}

// NormalizeState ensures we return a 2-letter state code if possible.
func NormalizeState(input string) string {
	upper := strings.ToUpper(strings.TrimSpace(input))
	if len(upper) == 2 {
		return upper
	}
	if code, ok := StateMap[upper]; ok {
		return code
	}
	return upper
}
