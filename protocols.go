package ipproto

import (
	"bytes"
	"encoding/csv"
	_ "embed"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

//go:embed protocol-numbers.csv
var embeddedCSV []byte

// Entry represents one row (or range) from the IANA protocol numbers CSV.
//
// The "Decimal" column can be either a single number (e.g. "6")
// or a range (e.g. "148-252"). For ranges, DecimalStart/DecimalEnd
// represent the inclusive range.
type Entry struct {
	DecimalStart int    // first number in the range
	DecimalEnd   int    // last number in the range (== DecimalStart for non-range)
	Keyword      string // "Keyword" column, short name, e.g. "TCP"
	Protocol     string // "Protocol" column, long name, e.g. "Transmission Control"
	IPv6ExtHdr   string // "IPv6 Extension Header" column (usually "Y" or empty)
	Reference    string // "Reference" column, raw text
}

var (
	mu             sync.RWMutex
	entries        []Entry
	byNumber       map[int]*Entry    // 6 -> TCP entry, 17 -> UDP, etc. (ranges expanded)
	byKeyword      map[string]*Entry // "TCP" -> entry
	byProtocolName map[string]*Entry // "transmission control" -> entry

	loadOnce sync.Once
	loadErr  error
)

// ensureLoaded parses the embedded CSV once on first use.
func ensureLoaded() error {
	loadOnce.Do(func() {
		if len(embeddedCSV) == 0 {
			loadErr = fmt.Errorf("ipproto: embedded CSV is empty; protocol-numbers.csv missing?")
			return
		}
		loadErr = loadFromReader(bytes.NewReader(embeddedCSV))
	})
	return loadErr
}

// LoadFromFile parses the given CSV file and overrides the embedded data.
// You don't need to call this for normal use.
func LoadFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("ipproto: open protocols csv: %w", err)
	}
	defer f.Close()
	return LoadFromReader(f)
}

// LoadFromReader parses protocol data from any io.Reader and overrides
// the embedded data. You don't need to call this for normal use.
func LoadFromReader(r io.Reader) error {
	mu.Lock()
	defer mu.Unlock()

	// Reset lazy loader and replace data.
	loadOnce = sync.Once{}
	loadErr = nil

	return loadFromReaderLocked(r)
}

// loadFromReader is the actual parser (no locking).
func loadFromReader(r io.Reader) error {
	mu.Lock()
	defer mu.Unlock()
	return loadFromReaderLocked(r)
}

// loadFromReaderLocked assumes mu is already locked.
func loadFromReaderLocked(r io.Reader) error {
	cr := csv.NewReader(r)
	cr.Comma = ','
	cr.Comment = '#'
	cr.FieldsPerRecord = -1 // allow variable columns per line

	records, err := cr.ReadAll()
	if err != nil {
		return fmt.Errorf("ipproto: read protocols csv: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("ipproto: protocols csv is empty")
	}

	entries = nil
	byNumber = make(map[int]*Entry)
	byKeyword = make(map[string]*Entry)
	byProtocolName = make(map[string]*Entry)

	// header: Decimal,Keyword,Protocol,IPv6 Extension Header,Reference
	startIdx := 1

	for i := startIdx; i < len(records); i++ {
		row := records[i]
		if len(row) == 0 {
			continue
		}

		for len(row) < 5 {
			row = append(row, "")
		}

		decField := strings.TrimSpace(row[0])
		keyword := strings.TrimSpace(row[1])
		proto := strings.TrimSpace(row[2])
		ipv6ext := strings.TrimSpace(row[3])
		ref := strings.TrimSpace(row[4])

		if decField == "" {
			continue
		}

		start, end, err := parseDecimalField(decField)
		if err != nil {
			// Non-numeric like "Unassigned", "Reserved", etc.
			continue
		}

		e := Entry{
			DecimalStart: start,
			DecimalEnd:   end,
			Keyword:      keyword,
			Protocol:     proto,
			IPv6ExtHdr:   ipv6ext,
			Reference:    ref,
		}

		entries = append(entries, e)
		idx := len(entries) - 1
		entryPtr := &entries[idx]

		// Fill byNumber for every value in the range
		for n := start; n <= end; n++ {
			byNumber[n] = entryPtr
		}

		// byKeyword (short name), case-insensitive; we store upper-case
		if keyword != "" {
			k := strings.ToUpper(keyword)
			if _, exists := byKeyword[k]; !exists {
				byKeyword[k] = entryPtr
			}
		}

		// byProtocolName (long name), normalized
		if proto != "" {
			p := normalizeProtoName(proto)
			if _, exists := byProtocolName[p]; !exists {
				byProtocolName[p] = entryPtr
			}
		}
	}

	return nil
}

// parseDecimalField parses the "Decimal" column, which may be:
//   - "6"
//   - "148-252"
//   - "Unassigned", "Reserved", etc. (returns error)
func parseDecimalField(s string) (start, end int, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, fmt.Errorf("empty decimal")
	}

	// Non-numeric things: "Unassigned", "Reserved", etc.
	if s[0] < '0' || s[0] > '9' {
		return 0, 0, fmt.Errorf("non-numeric decimal: %q", s)
	}

	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("invalid range %q", s)
		}
		a := strings.TrimSpace(parts[0])
		b := strings.TrimSpace(parts[1])
		start, err1 := strconv.Atoi(a)
		end, err2 := strconv.Atoi(b)
		if err1 != nil || err2 != nil {
			return 0, 0, fmt.Errorf("invalid range %q", s)
		}
		if end < start {
			return 0, 0, fmt.Errorf("range end < start: %q", s)
		}
		return start, end, nil
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid decimal %q", s)
	}
	return n, n, nil
}

// normalizeProtoName normalizes the "Protocol" (long name) field for lookup.
func normalizeProtoName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// LookupByNumber returns the Entry for a given protocol number (0–255).
//
// If not found, ok will be false.
func LookupByNumber(n int) (*Entry, bool) {
	if err := ensureLoaded(); err != nil {
		return nil, false
	}

	mu.RLock()
	defer mu.RUnlock()
	if byNumber == nil {
		return nil, false
	}
	e, ok := byNumber[n]
	return e, ok
}

// LookupDecimal returns the Decimal value for a given name.
//
// "name" can be either:
//   - the Keyword (short name, e.g. "TCP")
//   - the Protocol (long name, e.g. "Transmission Control")
//
// For ranges like "148-252", this returns DecimalStart.
func LookupDecimal(name string) (int, bool) {
	if err := ensureLoaded(); err != nil {
		return 0, false
	}

	mu.RLock()
	defer mu.RUnlock()
	if byKeyword == nil && byProtocolName == nil {
		return 0, false
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false
	}

	// Try Keyword (short name) first
	if e, ok := byKeyword[strings.ToUpper(name)]; ok {
		return e.DecimalStart, true
	}

	// Then try Protocol (long name)
	if e, ok := byProtocolName[normalizeProtoName(name)]; ok {
		return e.DecimalStart, true
	}

	return 0, false
}

// LookupKeyword returns the short name (Keyword) for a protocol number,
// e.g. 6 -> "TCP".
func LookupKeyword(n int) (string, bool) {
	e, ok := LookupByNumber(n)
	if !ok || e == nil || e.Keyword == "" {
		return "", false
	}
	return e.Keyword, true
}

// LookupProtocolName returns the long "Protocol" field for a protocol number,
// e.g. 6 -> "Transmission Control".
func LookupProtocolName(n int) (string, bool) {
	e, ok := LookupByNumber(n)
	if !ok || e == nil || e.Protocol == "" {
		return "", false
	}
	return e.Protocol, true
}


