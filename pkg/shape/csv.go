package shape

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const csvMaxSampleRows = 100

// Format detection regex patterns (same as JSON field stats).
var (
	csvUUIDRegex    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	csvISO8601Regex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}:\d{2})?`)
	csvURLRegex     = regexp.MustCompile(`^https?://`)
	csvEmailRegex   = regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`)
)

// ExtractCSVColumns parses a CSV body and detects column types and formats.
// Uses the first row as headers. Falls back to generated column names if
// the first row appears to be data.
func ExtractCSVColumns(body []byte) (*CSVColumnStats, error) {
	reader := csv.NewReader(bytes.NewReader(body))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // Allow variable field counts

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV parse error: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV")
	}

	// Determine if first row is headers
	headers := records[0]
	dataRows := records[1:]
	hasHeaders := !looksLikeData(headers, dataRows)

	if !hasHeaders {
		// Generate column names
		dataRows = records
		headers = make([]string, len(records[0]))
		for i := range headers {
			headers[i] = fmt.Sprintf("col_%d", i)
		}
	}

	// Limit sample rows
	if len(dataRows) > csvMaxSampleRows {
		dataRows = dataRows[:csvMaxSampleRows]
	}

	// Analyze each column
	columns := make([]CSVColumn, len(headers))
	for i, name := range headers {
		columns[i] = analyzeCSVColumn(name, i, dataRows)
	}

	return &CSVColumnStats{
		Columns:    columns,
		RowCount:   len(records) - boolToInt(hasHeaders),
		HasHeaders: hasHeaders,
	}, nil
}

// analyzeCSVColumn analyzes a single column across all data rows.
func analyzeCSVColumn(name string, colIdx int, rows [][]string) CSVColumn {
	col := CSVColumn{
		Name: name,
		Type: "string",
	}

	var values []string
	emptyCount := 0

	for _, row := range rows {
		if colIdx >= len(row) {
			emptyCount++
			continue
		}
		val := strings.TrimSpace(row[colIdx])
		if val == "" {
			emptyCount++
			continue
		}
		values = append(values, val)
	}

	totalRows := len(rows)
	if totalRows > 0 {
		col.EmptyFrequency = float64(emptyCount) / float64(totalRows)
	}

	// Collect examples (up to 3)
	seen := make(map[string]bool)
	for _, v := range values {
		if !seen[v] && len(col.Examples) < 3 {
			col.Examples = append(col.Examples, v)
			seen[v] = true
		}
	}

	if len(values) == 0 {
		return col
	}

	// Detect type
	col.Type = detectCSVColumnType(values)

	// Detect format for string columns
	if col.Type == "string" && len(values) >= 5 {
		col.Format, col.EnumValues = detectCSVFormat(values)
	}

	return col
}

// detectCSVColumnType determines the type of a column based on its values.
func detectCSVColumnType(values []string) string {
	allNumber := true
	allBool := true

	for _, v := range values {
		if allNumber {
			_, err := strconv.ParseFloat(v, 64)
			if err != nil {
				allNumber = false
			}
		}
		if allBool {
			lower := strings.ToLower(v)
			if lower != "true" && lower != "false" && lower != "0" && lower != "1" {
				allBool = false
			}
		}
		if !allNumber && !allBool {
			break
		}
	}

	if allNumber {
		return "number"
	}
	if allBool {
		return "boolean"
	}
	return "string"
}

// detectCSVFormat detects common formats for string column values.
func detectCSVFormat(values []string) (string, []string) {
	if allMatch(values, csvUUIDRegex) {
		return "uuid", nil
	}
	if allMatch(values, csvISO8601Regex) {
		return "iso8601", nil
	}
	if allMatch(values, csvURLRegex) {
		return "url", nil
	}
	if allMatch(values, csvEmailRegex) {
		return "email", nil
	}

	// Enum detection: <=10 distinct values
	distinct := make(map[string]bool)
	for _, v := range values {
		distinct[v] = true
	}
	if len(distinct) <= 10 {
		enumValues := make([]string, 0, len(distinct))
		for v := range distinct {
			enumValues = append(enumValues, v)
		}
		sort.Strings(enumValues)
		return "enum", enumValues
	}

	return "", nil
}

// allMatch returns true if all values match the regex pattern.
func allMatch(values []string, pattern *regexp.Regexp) bool {
	for _, v := range values {
		if !pattern.MatchString(v) {
			return false
		}
	}
	return true
}

// ExtractCSVColumnsMerged parses multiple CSV bodies and combines their rows
// for more accurate type and format detection.
func ExtractCSVColumnsMerged(bodies [][]byte) (*CSVColumnStats, error) {
	if len(bodies) == 1 {
		return ExtractCSVColumns(bodies[0])
	}

	// Parse all bodies, collect headers from the first valid body,
	// and combine data rows from all bodies.
	var headers []string
	var allDataRows [][]string
	hasHeaders := true
	totalRowCount := 0
	parsed := 0

	for _, body := range bodies {
		reader := csv.NewReader(bytes.NewReader(body))
		reader.LazyQuotes = true
		reader.FieldsPerRecord = -1

		records, err := reader.ReadAll()
		if err != nil || len(records) == 0 {
			continue
		}
		parsed++

		h := records[0]
		dataRows := records[1:]
		hd := !looksLikeData(h, dataRows)

		if headers == nil {
			headers = h
			hasHeaders = hd
			if !hd {
				headers = make([]string, len(records[0]))
				for i := range headers {
					headers[i] = fmt.Sprintf("col_%d", i)
				}
				dataRows = records
			}
		} else if !hd {
			dataRows = records
		}

		totalRowCount += len(dataRows)
		allDataRows = append(allDataRows, dataRows...)
	}

	if headers == nil {
		return nil, fmt.Errorf("no valid CSV samples found")
	}

	// Limit sample rows
	if len(allDataRows) > csvMaxSampleRows {
		allDataRows = allDataRows[:csvMaxSampleRows]
	}

	columns := make([]CSVColumn, len(headers))
	for i, name := range headers {
		columns[i] = analyzeCSVColumn(name, i, allDataRows)
	}

	return &CSVColumnStats{
		Columns:     columns,
		RowCount:    totalRowCount,
		HasHeaders:  hasHeaders,
		SampleCount: parsed,
	}, nil
}

// looksLikeData heuristically determines if the first row appears to be data
// rather than headers. If most values parse as numbers/dates matching subsequent
// rows, it's likely data.
func looksLikeData(firstRow []string, dataRows [][]string) bool {
	if len(dataRows) == 0 {
		return false
	}

	numericCount := 0
	for _, val := range firstRow {
		_, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err == nil {
			numericCount++
		}
	}

	// If more than half the first row values are numeric, treat as data
	return numericCount > len(firstRow)/2
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
