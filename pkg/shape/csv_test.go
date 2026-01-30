package shape

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractCSVColumns_BasicWithHeaders(t *testing.T) {
	body := []byte("name,age,active\nAlice,30,true\nBob,25,false\nCharlie,35,true\n")

	cols, err := ExtractCSVColumns(body)
	require.NoError(t, err)
	assert.True(t, cols.HasHeaders)
	assert.Equal(t, 3, cols.RowCount)
	require.Len(t, cols.Columns, 3)

	assert.Equal(t, "name", cols.Columns[0].Name)
	assert.Equal(t, "string", cols.Columns[0].Type)

	assert.Equal(t, "age", cols.Columns[1].Name)
	assert.Equal(t, "number", cols.Columns[1].Type)

	assert.Equal(t, "active", cols.Columns[2].Name)
	assert.Equal(t, "boolean", cols.Columns[2].Type)
}

func TestExtractCSVColumns_TypeDetection(t *testing.T) {
	body := []byte("val\n42\n100\n3.14\n0\n-5\n")

	cols, err := ExtractCSVColumns(body)
	require.NoError(t, err)
	assert.Equal(t, "number", cols.Columns[0].Type)
}

func TestExtractCSVColumns_BooleanDetection(t *testing.T) {
	body := []byte("flag\ntrue\nfalse\ntrue\nfalse\n")

	cols, err := ExtractCSVColumns(body)
	require.NoError(t, err)
	assert.Equal(t, "boolean", cols.Columns[0].Type)
}

func TestExtractCSVColumns_DateFormat(t *testing.T) {
	body := []byte("date\n2024-01-15\n2024-02-20\n2024-03-25\n2024-04-10\n2024-05-05\n")

	cols, err := ExtractCSVColumns(body)
	require.NoError(t, err)
	assert.Equal(t, "string", cols.Columns[0].Type)
	assert.Equal(t, "iso8601", cols.Columns[0].Format)
}

func TestExtractCSVColumns_NoHeaderFallback(t *testing.T) {
	// All numeric first row suggests data, not headers
	body := []byte("1,2,3\n4,5,6\n7,8,9\n")

	cols, err := ExtractCSVColumns(body)
	require.NoError(t, err)
	assert.False(t, cols.HasHeaders)
	assert.Equal(t, "col_0", cols.Columns[0].Name)
	assert.Equal(t, "col_1", cols.Columns[1].Name)
}

func TestExtractCSVColumns_EmptyValues(t *testing.T) {
	body := []byte("name,email\nAlice,alice@example.com\nBob,\nCharlie,charlie@example.com\n")

	cols, err := ExtractCSVColumns(body)
	require.NoError(t, err)
	assert.InDelta(t, 0.333, cols.Columns[1].EmptyFrequency, 0.01)
}

func TestExtractCSVColumns_TabSeparated(t *testing.T) {
	// Note: the CSV reader handles tab-separated if told to, but
	// our code uses default comma delimiter. TSV detection would
	// need additional handling -- for now test with comma.
	body := []byte("a,b\n1,2\n3,4\n")

	cols, err := ExtractCSVColumns(body)
	require.NoError(t, err)
	assert.Len(t, cols.Columns, 2)
}

func TestExtractCSVColumns_Empty(t *testing.T) {
	body := []byte("")
	_, err := ExtractCSVColumns(body)
	assert.Error(t, err)
}

func TestExtractCSVColumnsMerged_CombinesRows(t *testing.T) {
	body1 := []byte("name,age\nAlice,30\nBob,25\n")
	body2 := []byte("name,age\nCharlie,35\nDiana,28\n")

	cols, err := ExtractCSVColumnsMerged([][]byte{body1, body2})
	require.NoError(t, err)
	assert.Equal(t, 4, cols.RowCount)
	assert.Equal(t, 2, cols.SampleCount)
	assert.True(t, cols.HasHeaders)
	require.Len(t, cols.Columns, 2)
	assert.Equal(t, "name", cols.Columns[0].Name)
	assert.Equal(t, "number", cols.Columns[1].Type)
}

func TestExtractCSVColumnsMerged_SkipsBadSamples(t *testing.T) {
	good := []byte("name,age\nAlice,30\n")
	bad := []byte("") // empty, will fail

	cols, err := ExtractCSVColumnsMerged([][]byte{good, bad})
	require.NoError(t, err)
	assert.Equal(t, 1, cols.SampleCount)
	assert.Equal(t, 1, cols.RowCount)
}

func TestExtractCSVColumnsMerged_BetterTypeDetection(t *testing.T) {
	// Each sample alone has too few rows for format detection,
	// but merged they have enough.
	body1 := []byte("id\n550e8400-e29b-41d4-a716-446655440000\naab3c4d5-e6f7-4890-abcd-ef1234567890\n")
	body2 := []byte("id\n12345678-1234-1234-1234-123456789abc\ndeadbeef-dead-beef-dead-beefdeadbeef\n")
	body3 := []byte("id\n00000000-0000-0000-0000-000000000001\n")

	cols, err := ExtractCSVColumnsMerged([][]byte{body1, body2, body3})
	require.NoError(t, err)
	assert.Equal(t, "uuid", cols.Columns[0].Format)
}
