package graphql

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRequestBody_StandardQuery(t *testing.T) {
	body := []byte(`{"query": "query GetUser($id: ID!) { user(id: $id) { id name email } }", "operationName": "GetUser", "variables": {"id": "123"}}`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)
	assert.False(t, result.IsBatched)
	require.Len(t, result.Operations, 1)

	op := result.Operations[0]
	assert.Equal(t, "GetUser", op.Name)
	assert.Equal(t, "query", op.Type)
	assert.Contains(t, op.Fields, "user")
	assert.True(t, op.HasVariables)
	assert.False(t, op.ParseFailed)
}

func TestParseRequestBody_Mutation(t *testing.T) {
	body := []byte(`{"query": "mutation CreateOrder($input: OrderInput!) { createOrder(input: $input) { id status } }"}`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)

	op := result.Operations[0]
	assert.Equal(t, "CreateOrder", op.Name)
	assert.Equal(t, "mutation", op.Type)
	assert.Contains(t, op.Fields, "createOrder")
}

func TestParseRequestBody_Subscription(t *testing.T) {
	body := []byte(`{"query": "subscription OnMessage { messageAdded { id text } }"}`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)

	op := result.Operations[0]
	assert.Equal(t, "OnMessage", op.Name)
	assert.Equal(t, "subscription", op.Type)
	assert.Contains(t, op.Fields, "messageAdded")
}

func TestParseRequestBody_Anonymous(t *testing.T) {
	body := []byte(`{"query": "{ user { id name } }"}`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)

	op := result.Operations[0]
	assert.Equal(t, "anonymous", op.Name)
	assert.Equal(t, "query", op.Type)
	assert.Contains(t, op.Fields, "user")
}

func TestParseRequestBody_AnonymousQueryKeyword(t *testing.T) {
	body := []byte(`{"query": "query { users { id } }"}`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)

	op := result.Operations[0]
	assert.Equal(t, "anonymous", op.Name)
	assert.Equal(t, "query", op.Type)
	assert.Contains(t, op.Fields, "users")
}

func TestParseRequestBody_Batched(t *testing.T) {
	body := []byte(`[
		{"query": "query GetUser { user { id } }", "operationName": "GetUser"},
		{"query": "mutation UpdateUser { updateUser { id } }", "operationName": "UpdateUser"}
	]`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)
	assert.True(t, result.IsBatched)
	require.Len(t, result.Operations, 2)

	assert.Equal(t, "GetUser", result.Operations[0].Name)
	assert.Equal(t, "query", result.Operations[0].Type)
	assert.Equal(t, 0, result.Operations[0].BatchIndex)

	assert.Equal(t, "UpdateUser", result.Operations[1].Name)
	assert.Equal(t, "mutation", result.Operations[1].Type)
	assert.Equal(t, 1, result.Operations[1].BatchIndex)
}

func TestParseRequestBody_UnparseableQuery(t *testing.T) {
	body := []byte(`{"query": "<<<not valid graphql>>>", "operationName": "FallbackOp"}`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)

	op := result.Operations[0]
	assert.Equal(t, "FallbackOp", op.Name)       // Falls back to operationName field
	assert.Equal(t, "query", op.Type)             // Default type
	assert.Equal(t, "<<<not valid graphql>>>", op.RawQuery)
	assert.False(t, op.ParseFailed) // scanQuery still returns ok=true for any non-empty input
}

func TestParseRequestBody_EmptyBody(t *testing.T) {
	_, err := ParseRequestBody([]byte(""))
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrEmpty))
	assert.True(t, IsNotGraphQL(err))

	// errors.As should extract ParseError
	var pe *ParseError
	assert.True(t, errors.As(err, &pe))
	assert.Equal(t, ErrEmpty, pe.Sentinel)
}

func TestParseRequestBody_NonJSON(t *testing.T) {
	_, err := ParseRequestBody([]byte("not json at all"))
	assert.Error(t, err)

	// Should be a ParseError wrapping a JSON error
	var pe *ParseError
	assert.True(t, errors.As(err, &pe))
	assert.Nil(t, pe.Sentinel) // JSON parse error, not a sentinel
	assert.NotNil(t, pe.Cause)
}

func TestParseRequestBody_NoQueryField(t *testing.T) {
	body := []byte(`{"data": "not graphql"}`)
	_, err := ParseRequestBody(body)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotGraphQL))
	assert.True(t, IsNotGraphQL(err))

	var pe *ParseError
	assert.True(t, errors.As(err, &pe))
	assert.Equal(t, ErrNotGraphQL, pe.Sentinel)
}

func TestParseRequestBody_OperationNameOnly(t *testing.T) {
	// Some clients send operationName without embedding it in the query
	body := []byte(`{"query": "{ user { id } }", "operationName": "GetUser"}`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)

	op := result.Operations[0]
	assert.Equal(t, "GetUser", op.Name) // operationName wins over parsed anonymous
}

func TestParseRequestBody_MultipleTopLevelFields(t *testing.T) {
	body := []byte(`{"query": "query Dashboard { user { id } orders { id } notifications { id } }"}`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)

	op := result.Operations[0]
	assert.Equal(t, "Dashboard", op.Name)
	assert.Len(t, op.Fields, 3)
	assert.Contains(t, op.Fields, "user")
	assert.Contains(t, op.Fields, "orders")
	assert.Contains(t, op.Fields, "notifications")
}

func TestScanQuery_SkipsGraphQLKeywords(t *testing.T) {
	// Ensure "fragment" and "on" at depth 1 are not treated as fields
	_, _, fields, ok := scanQuery("query Test { user { ...UserFields } }")
	assert.True(t, ok)
	assert.Contains(t, fields, "user")
}

func TestParseRequestBody_WhitespaceBody(t *testing.T) {
	_, err := ParseRequestBody([]byte("   \n\t  "))
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrEmpty))
	assert.True(t, IsNotGraphQL(err))
}

// ---------------------------------------------------------------------------
// IsGraphQLBody tests
// ---------------------------------------------------------------------------

func TestIsGraphQLBody_ValidQuery(t *testing.T) {
	assert.True(t, IsGraphQLBody([]byte(`{"query": "{ user { id } }"}`)))
}

func TestIsGraphQLBody_ValidMutation(t *testing.T) {
	assert.True(t, IsGraphQLBody([]byte(`{"query": "mutation { createUser { id } }", "operationName": "CreateUser"}`)))
}

func TestIsGraphQLBody_BatchedQuery(t *testing.T) {
	assert.True(t, IsGraphQLBody([]byte(`[{"query": "{ user { id } }"}]`)))
}

func TestIsGraphQLBody_EmptyQuery(t *testing.T) {
	assert.False(t, IsGraphQLBody([]byte(`{"query": ""}`)))
}

func TestIsGraphQLBody_NoQueryField(t *testing.T) {
	assert.False(t, IsGraphQLBody([]byte(`{"data": "hello"}`)))
}

func TestIsGraphQLBody_NotJSON(t *testing.T) {
	assert.False(t, IsGraphQLBody([]byte("not json")))
}

func TestIsGraphQLBody_EmptyBody(t *testing.T) {
	assert.False(t, IsGraphQLBody([]byte("")))
}

func TestIsGraphQLBody_EmptyBatch(t *testing.T) {
	assert.False(t, IsGraphQLBody([]byte(`[]`)))
}

// ---------------------------------------------------------------------------
// Error type tests
// ---------------------------------------------------------------------------

func TestParseError_ErrorsIs(t *testing.T) {
	_, err := ParseRequestBody([]byte(""))
	assert.True(t, errors.Is(err, ErrEmpty))
	assert.False(t, errors.Is(err, ErrNotGraphQL))

	_, err = ParseRequestBody([]byte(`{"data": "x"}`))
	assert.True(t, errors.Is(err, ErrNotGraphQL))
	assert.False(t, errors.Is(err, ErrEmpty))
}

func TestParseError_ErrorsAs(t *testing.T) {
	_, err := ParseRequestBody([]byte("bad json!!!"))
	var pe *ParseError
	require.True(t, errors.As(err, &pe))
	assert.Contains(t, pe.Message, "invalid JSON")
	assert.NotNil(t, pe.Cause) // JSON syntax error
}
