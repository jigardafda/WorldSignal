// Package countries provides ISO 3166-1 reference data (name, flag, currency,
// capital and the capital's geolocation) served to the frontend so country
// fields are dropdowns rather than free text.
package countries

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
)

// Country is one reference entry. Flag is derived from Code at load time.
type Country struct {
	Code       string  `json:"code"` // ISO 3166-1 alpha-2
	Name       string  `json:"name"`
	Flag       string  `json:"flag"`
	Capital    string  `json:"capital"`
	Currency   string  `json:"currency"` // ISO 4217
	CapitalLat float64 `json:"capitalLat"`
	CapitalLng float64 `json:"capitalLng"`
}

//go:embed data.json
var raw []byte

var (
	list  []Country
	byKey map[string]Country
)

func init() {
	var data []Country
	if err := json.Unmarshal(raw, &data); err != nil {
		panic("countries: bad embedded data: " + err.Error())
	}
	byKey = make(map[string]Country, len(data))
	for i := range data {
		data[i].Flag = flagOf(data[i].Code)
		byKey[data[i].Code] = data[i] // dedup by code (last wins)
	}
	list = make([]Country, 0, len(byKey))
	for _, c := range byKey {
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
}

// All returns every country, sorted by name.
func All() []Country { return list }

// Get returns a country by ISO alpha-2 code (case-insensitive), ok=false if unknown.
func Get(code string) (Country, bool) {
	c, ok := byKey[strings.ToUpper(strings.TrimSpace(code))]
	return c, ok
}

// flagOf converts a 2-letter ISO code into its flag emoji using Unicode
// regional indicator symbols (A→🇦 … Z→🇿).
func flagOf(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return ""
	}
	const base = 0x1F1E6 // regional indicator 'A'
	return string(rune(base+int(code[0]-'A'))) + string(rune(base+int(code[1]-'A')))
}
