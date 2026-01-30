package shape

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/usestring/powhttp-mcp/pkg/contenttype"
	"github.com/usestring/powhttp-mcp/pkg/jsonschema"
	"github.com/usestring/powhttp-mcp/pkg/textquery"
)

// Engine dispatches shape analysis by content type.
// It follows the same unified-engine pattern as textquery.Engine.
type Engine struct{}

// NewEngine creates a new shape analysis engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Analyze performs shape analysis on a set of bodies with the given content type.
// It dispatches to the appropriate format-specific analyzer based on content type.
func (e *Engine) Analyze(bodies [][]byte, ct string) (*Result, error) {
	if len(bodies) == 0 {
		return nil, fmt.Errorf("no bodies to analyze")
	}

	category := contenttype.Classify(ct)

	switch category {
	case contenttype.JSON:
		return e.analyzeJSON(bodies)
	case contenttype.YAML:
		return e.analyzeYAML(bodies)
	case contenttype.XML:
		return e.analyzeXML(bodies)
	case contenttype.CSV:
		return e.analyzeCSV(bodies)
	case contenttype.HTML:
		return e.analyzeHTML(bodies)
	case contenttype.Form:
		return e.analyzeForm(bodies)
	case contenttype.Binary:
		return &Result{
			ContentCategory: "binary",
			Skipped:         true,
			SkipReason:      fmt.Sprintf("binary content type: %s", ct),
		}, nil
	default:
		return &Result{
			ContentCategory: string(category),
			Skipped:         true,
			SkipReason:      fmt.Sprintf("unsupported content type: %s", ct),
		}, nil
	}
}

// analyzeJSON infers a JSON schema and computes field statistics.
func (e *Engine) analyzeJSON(bodies [][]byte) (*Result, error) {
	inferred, err := jsonschema.Infer(bodies...)
	if err != nil {
		return nil, fmt.Errorf("JSON schema inference failed: %w", err)
	}
	if inferred == nil {
		return nil, fmt.Errorf("no valid JSON samples found")
	}

	stats := jsonschema.ComputeFieldStats(inferred.Schema, bodies)

	return &Result{
		ContentCategory: "json",
		Schema:          inferred.Schema,
		FieldStats:      stats,
		SampleCount:     inferred.SampleCount,
		AllMatch:        inferred.AllMatch,
	}, nil
}

// analyzeYAML converts YAML to JSON and delegates to the JSON analyzer.
func (e *Engine) analyzeYAML(bodies [][]byte) (*Result, error) {
	jsonBodies := make([][]byte, 0, len(bodies))

	for _, body := range bodies {
		var yamlData any
		if err := yaml.Unmarshal(body, &yamlData); err != nil {
			continue
		}

		jsonBytes, err := json.Marshal(textquery.ConvertYAMLToJSON(yamlData))
		if err != nil {
			continue
		}

		jsonBodies = append(jsonBodies, jsonBytes)
	}

	if len(jsonBodies) == 0 {
		return nil, fmt.Errorf("no valid YAML samples found")
	}

	result, err := e.analyzeJSON(jsonBodies)
	if err != nil {
		return nil, err
	}

	result.ContentCategory = "yaml"
	return result, nil
}

// analyzeXML extracts the XML element hierarchy from all samples,
// merging hierarchies so that elements appearing in any sample are included.
func (e *Engine) analyzeXML(bodies [][]byte) (*Result, error) {
	var merged *XMLElementHierarchy
	parsed := 0

	for _, body := range bodies {
		h, err := ExtractXMLHierarchy(body)
		if err != nil {
			continue
		}
		parsed++
		if merged == nil {
			merged = h
		} else {
			mergeXMLHierarchy(merged, h)
		}
	}

	if merged == nil {
		return nil, fmt.Errorf("no valid XML samples found")
	}
	merged.SampleCount = parsed

	return &Result{
		ContentCategory: "xml",
		XMLHierarchy:    merged,
		SampleCount:     parsed,
	}, nil
}

// analyzeCSV detects CSV column types and formats.
// When multiple bodies are provided, rows from all samples are combined
// for more accurate type and format detection.
func (e *Engine) analyzeCSV(bodies [][]byte) (*Result, error) {
	columns, err := ExtractCSVColumnsMerged(bodies)
	if err != nil {
		return nil, fmt.Errorf("CSV column extraction failed: %w", err)
	}

	return &Result{
		ContentCategory: "csv",
		CSVColumns:      columns,
		SampleCount:     columns.SampleCount,
	}, nil
}

// analyzeHTML extracts the HTML DOM outline from all samples,
// merging tag counts, element IDs, forms, and meta tags across samples.
func (e *Engine) analyzeHTML(bodies [][]byte) (*Result, error) {
	var merged *HTMLDOMOutline
	parsed := 0

	for _, body := range bodies {
		outline, err := ExtractHTMLOutline(body)
		if err != nil {
			continue
		}
		parsed++
		if merged == nil {
			merged = outline
		} else {
			mergeHTMLOutline(merged, outline)
		}
	}

	if merged == nil {
		return nil, fmt.Errorf("no valid HTML samples found")
	}
	merged.SampleCount = parsed

	return &Result{
		ContentCategory: "html",
		HTMLOutline:     merged,
		SampleCount:     parsed,
	}, nil
}

// analyzeForm extracts form keys and their statistics.
func (e *Engine) analyzeForm(bodies [][]byte) (*Result, error) {
	keys := extractFormKeys(bodies)
	if len(keys) == 0 {
		return nil, fmt.Errorf("no form keys found")
	}

	return &Result{
		ContentCategory: "form",
		FormKeys:        keys,
		SampleCount:     len(bodies),
	}, nil
}

// extractFormKeys parses form-urlencoded bodies and returns key statistics.
func extractFormKeys(bodies [][]byte) []FormKeyStat {
	totalBodies := len(bodies)
	keyPresence := make(map[string]int)
	keyExamples := make(map[string][]string)

	for _, body := range bodies {
		values, err := url.ParseQuery(string(body))
		if err != nil {
			continue
		}

		for key, vals := range values {
			keyPresence[key]++
			if len(keyExamples[key]) < 3 && len(vals) > 0 {
				keyExamples[key] = append(keyExamples[key], vals[0])
			}
		}
	}

	stats := make([]FormKeyStat, 0, len(keyPresence))
	for key, count := range keyPresence {
		stats = append(stats, FormKeyStat{
			Key:       key,
			Frequency: float64(count) / float64(totalBodies),
			Examples:  keyExamples[key],
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Key < stats[j].Key
	})

	return stats
}
