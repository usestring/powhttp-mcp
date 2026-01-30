package jsonschema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: This file tests the package's own types directly (FieldStat, ComputeFieldStats)
// without needing to import the external jsonschema package.

func TestComputeFieldStats_BasicFields(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"id": 1, "name": "Alice", "active": true}`),
		[]byte(`{"id": 2, "name": "Bob", "active": false}`),
		[]byte(`{"id": 3, "name": "Charlie", "active": true}`),
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)
	require.NotNil(t, inferred)

	stats := ComputeFieldStats(inferred.Schema, samples)
	require.NotNil(t, stats)

	// Find stats by path
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	// All fields present in all samples
	assert.Equal(t, 1.0, byPath["id"].Frequency)
	assert.True(t, byPath["id"].Required)
	assert.False(t, byPath["id"].Nullable)
	assert.Equal(t, "integer", byPath["id"].Type)

	assert.Equal(t, 1.0, byPath["name"].Frequency)
	assert.True(t, byPath["name"].Required)
	assert.Equal(t, "string", byPath["name"].Type)
	assert.Equal(t, 3, byPath["name"].DistinctCount)

	assert.Equal(t, 1.0, byPath["active"].Frequency)
	assert.True(t, byPath["active"].Required)
	assert.Equal(t, "boolean", byPath["active"].Type)
}

func TestComputeFieldStats_OptionalFields(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"id": 1, "name": "Alice"}`),
		[]byte(`{"id": 2}`),
		[]byte(`{"id": 3, "name": "Charlie"}`),
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	assert.Equal(t, 1.0, byPath["id"].Frequency)
	assert.True(t, byPath["id"].Required)

	// name is present in 2 of 3
	assert.InDelta(t, 0.666, byPath["name"].Frequency, 0.01)
	assert.False(t, byPath["name"].Required)
}

func TestComputeFieldStats_NullableFields(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"id": 1, "email": "alice@example.com"}`),
		[]byte(`{"id": 2, "email": null}`),
		[]byte(`{"id": 3, "email": "charlie@example.com"}`),
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	assert.Equal(t, 1.0, byPath["email"].Frequency)
	assert.True(t, byPath["email"].Nullable)
	assert.False(t, byPath["email"].Required) // nullable means not required
	assert.Equal(t, 2, byPath["email"].DistinctCount)
}

func TestComputeFieldStats_NestedObjects(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"user": {"id": 1, "name": "Alice"}}`),
		[]byte(`{"user": {"id": 2, "name": "Bob"}}`),
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	assert.Contains(t, byPath, "user")
	assert.Contains(t, byPath, "user.id")
	assert.Contains(t, byPath, "user.name")
	assert.Equal(t, "object", byPath["user"].Type)
	assert.Equal(t, "integer", byPath["user.id"].Type)
}

func TestComputeFieldStats_ArrayItems(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"items": [{"id": 1, "title": "A"}, {"id": 2, "title": "B"}]}`),
		[]byte(`{"items": [{"id": 3, "title": "C"}]}`),
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	assert.Contains(t, byPath, "items")
	assert.Contains(t, byPath, "items[].id")
	assert.Contains(t, byPath, "items[].title")
	assert.Equal(t, "array", byPath["items"].Type)
}

func TestComputeFieldStats_FormatDetection_UUID(t *testing.T) {
	samples := make([][]byte, 6)
	uuids := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"7c9e6679-7425-40de-944b-e07fc1f90ae7",
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"12345678-1234-1234-1234-123456789abc",
	}
	for i, uuid := range uuids {
		samples[i] = []byte(`{"id": "` + uuid + `"}`)
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	assert.Equal(t, "uuid", byPath["id"].Format)
}

func TestComputeFieldStats_FormatDetection_ISO8601(t *testing.T) {
	samples := make([][]byte, 5)
	dates := []string{
		"2024-01-15T10:30:00Z",
		"2024-02-20T14:00:00Z",
		"2024-03-25T09:15:00Z",
		"2024-04-10T16:45:00Z",
		"2024-05-05T11:00:00Z",
	}
	for i, d := range dates {
		samples[i] = []byte(`{"created_at": "` + d + `"}`)
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	assert.Equal(t, "iso8601", byPath["created_at"].Format)
}

func TestComputeFieldStats_FormatDetection_URL(t *testing.T) {
	samples := make([][]byte, 5)
	urls := []string{
		"https://example.com/page1",
		"https://example.com/page2",
		"http://test.org/api",
		"https://foo.bar/baz",
		"https://a.b.c/d",
	}
	for i, u := range urls {
		samples[i] = []byte(`{"url": "` + u + `"}`)
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	assert.Equal(t, "url", byPath["url"].Format)
}

func TestComputeFieldStats_FormatDetection_Email(t *testing.T) {
	samples := make([][]byte, 5)
	emails := []string{
		"alice@example.com",
		"bob@test.org",
		"charlie@foo.bar",
		"dave@company.co",
		"eve@domain.net",
	}
	for i, e := range emails {
		samples[i] = []byte(`{"email": "` + e + `"}`)
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	assert.Equal(t, "email", byPath["email"].Format)
}

func TestComputeFieldStats_EnumDetection(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"status": "active"}`),
		[]byte(`{"status": "inactive"}`),
		[]byte(`{"status": "pending"}`),
		[]byte(`{"status": "active"}`),
		[]byte(`{"status": "inactive"}`),
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	assert.Equal(t, "enum", byPath["status"].Format)
	assert.ElementsMatch(t, []string{"active", "inactive", "pending"}, byPath["status"].EnumValues)
}

func TestComputeFieldStats_InsufficientSamplesSkipsFormat(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"id": "550e8400-e29b-41d4-a716-446655440000"}`),
		[]byte(`{"id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8"}`),
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	// Only 2 samples, format detection requires 5
	assert.Empty(t, byPath["id"].Format)
}

func TestComputeFieldStats_DepthLimit(t *testing.T) {
	// Create deeply nested JSON
	deep := map[string]any{
		"l1": map[string]any{
			"l2": map[string]any{
				"l3": map[string]any{
					"l4": map[string]any{
						"l5": map[string]any{
							"l6": map[string]any{
								"value": "deep",
							},
						},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(deep)
	samples := [][]byte{data}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)

	// Should have stats up to l5 but l6 should be truncated
	paths := make(map[string]bool)
	for _, s := range stats {
		paths[s.Path] = true
	}

	assert.True(t, paths["l1"])
	assert.True(t, paths["l1.l2"])
	assert.True(t, paths["l1.l2.l3"])
	assert.True(t, paths["l1.l2.l3.l4"])
	assert.True(t, paths["l1.l2.l3.l4.l5"])
}

func TestComputeFieldStats_NilSchema(t *testing.T) {
	stats := ComputeFieldStats(nil, [][]byte{[]byte(`{"a": 1}`)})
	assert.Nil(t, stats)
}

func TestComputeFieldStats_EmptySamples(t *testing.T) {
	inferred, err := Infer([]byte(`{"a": 1}`))
	require.NoError(t, err)
	stats := ComputeFieldStats(inferred.Schema, nil)
	assert.Nil(t, stats)
}

func TestComputeFieldStats_Examples(t *testing.T) {
	samples := [][]byte{
		[]byte(`{"name": "Alice"}`),
		[]byte(`{"name": "Bob"}`),
		[]byte(`{"name": "Charlie"}`),
		[]byte(`{"name": "Dave"}`),
		[]byte(`{"name": "Eve"}`),
	}

	inferred, err := Infer(samples...)
	require.NoError(t, err)

	stats := ComputeFieldStats(inferred.Schema, samples)
	byPath := make(map[string]FieldStat)
	for _, s := range stats {
		byPath[s.Path] = s
	}

	// Should have at most 3 examples
	assert.LessOrEqual(t, len(byPath["name"].Examples), 3)
	assert.Greater(t, len(byPath["name"].Examples), 0)
}
