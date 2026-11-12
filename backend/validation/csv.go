package validation

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	MaxCSVBytes        = 10 * 1024 * 1024 // 10 MB
	MaxCSVRows         = 10_000
	MaxCSVColumnLength = 1_000
)

// ParseCSV decodes UTF-8 (with optional BOM) or Latin-1/ISO-8859-1 CSV/TSV/semicolon text using RFC 4180 parsing.
// It enforces MaxCSVBytes, MaxCSVRows, and MaxCSVColumnLength.
func ParseCSV(raw string, autoDetectDelimiter bool) ([]string, [][]string, error) {
	if len(raw) > MaxCSVBytes {
		return nil, nil, fmt.Errorf("CSV payload exceeds maximum size of %d bytes", MaxCSVBytes)
	}

	cleaned := decodeCSVText(raw)
	if strings.TrimSpace(cleaned) == "" {
		return nil, nil, errors.New("CSV content is empty")
	}

	comma := ','
	if autoDetectDelimiter {
		comma = detectDelimiter(cleaned)
	}

	reader := csv.NewReader(strings.NewReader(cleaned))
	reader.Comma = comma
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = false
	reader.LazyQuotes = true

	var headers []string
	var rows [][]string
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("parse CSV row %d: %w", len(rows)+2, err)
		}
		if headers == nil {
			headers = make([]string, len(rec))
			for i, h := range rec {
				headers[i] = strings.TrimSpace(strings.TrimPrefix(h, "\uFEFF"))
			}
			continue
		}
		if isBlankRecord(rec) {
			continue
		}
		if len(rows) >= MaxCSVRows {
			return nil, nil, fmt.Errorf("CSV exceeds maximum row limit of %d", MaxCSVRows)
		}
		for i, col := range rec {
			if utf8.RuneCountInString(col) > MaxCSVColumnLength {
				return nil, nil, fmt.Errorf("row %d column %d exceeds max length of %d characters", len(rows)+2, i+1, MaxCSVColumnLength)
			}
			rec[i] = strings.TrimSpace(col)
		}
		rows = append(rows, rec)
	}

	if len(headers) == 0 {
		return nil, nil, errors.New("CSV header row is missing")
	}
	return headers, rows, nil
}

// EncodeCSV generates an RFC 4180 CSV string with UTF-8 BOM and CRLF line endings for clean Windows/Excel opening.
func EncodeCSV(headers []string, rows [][]string) (string, error) {
	var buf bytes.Buffer
	buf.WriteString("\uFEFF")

	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	if err := w.Write(sanitizeCells(headers)); err != nil {
		return "", err
	}
	for _, r := range rows {
		if err := w.Write(sanitizeCells(r)); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// HeaderIndex builds a lowercase, whitespace/underscore/hyphen-normalized lookup map from CSV header names to column indices.
func HeaderIndex(headers []string) map[string]int {
	idx := make(map[string]int, len(headers)*2)
	for i, h := range headers {
		trimmed := strings.TrimSpace(h)
		lower := strings.ToLower(trimmed)
		if _, exists := idx[lower]; !exists {
			idx[lower] = i
		}
		norm := NormalizeHeaderKey(trimmed)
		if _, exists := idx[norm]; !exists {
			idx[norm] = i
		}
	}
	return idx
}

// NormalizeHeaderKey canonicalizes column headers across camelCase, snake_case, and space-separated variants.
func NormalizeHeaderKey(h string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(h)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CellByAliases fetches the first non-empty matching column value by normalized alias keys.
func CellByAliases(row []string, idx map[string]int, aliases ...string) string {
	for _, alias := range aliases {
		if pos, ok := idx[NormalizeHeaderKey(alias)]; ok && pos < len(row) {
			return strings.TrimSpace(row[pos])
		}
	}
	return ""
}

func decodeCSVText(raw string) string {
	cleaned := strings.TrimPrefix(raw, "\uFEFF")
	if !utf8.ValidString(cleaned) {
		// Fall back to ISO-8859-1 byte-to-rune widening.
		rawBytes := []byte(cleaned)
		var b strings.Builder
		b.Grow(len(rawBytes))
		for _, by := range rawBytes {
			b.WriteRune(rune(by))
		}
		cleaned = b.String()
	}
	return cleaned
}

func detectDelimiter(text string) rune {
	firstLine := text
	if nl := strings.IndexByte(text, '\n'); nl >= 0 {
		firstLine = text[:nl]
	}
	tabs := strings.Count(firstLine, "\t")
	semis := strings.Count(firstLine, ";")
	commas := strings.Count(firstLine, ",")
	switch {
	case tabs >= semis && tabs > commas && tabs > 0:
		return '\t'
	case semis > commas && semis > 0:
		return ';'
	default:
		return ','
	}
}

func isBlankRecord(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func sanitizeCells(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		// Guard against spreadsheet formula injection while preserving plain negative numbers.
		trimmed := strings.TrimSpace(c)
		if len(trimmed) > 0 && (trimmed[0] == '=' || trimmed[0] == '+' || trimmed[0] == '@') {
			out[i] = "'" + c
			continue
		}
		out[i] = c
	}
	return out
}
