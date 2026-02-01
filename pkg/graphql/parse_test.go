package graphql

import (
	"encoding/json"
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

// ---------------------------------------------------------------------------
// Complex GraphQL parsing tests
// ---------------------------------------------------------------------------

func TestExtractTopLevelFields_SkipsArguments(t *testing.T) {
	// Arguments inside () should not be extracted as fields
	fields := extractTopLevelFields(`{ user(id: "123") { name email } }`)
	assert.Equal(t, []string{"user"}, fields)
}

func TestExtractTopLevelFields_SkipsNestedArguments(t *testing.T) {
	// Complex arguments with nested objects
	fields := extractTopLevelFields(`{ search(filter: {category: "books", limit: 10}, sort: DESC) { items { id title } totalCount } }`)
	assert.Equal(t, []string{"search"}, fields)
}

func TestExtractTopLevelFields_SkipsVariableArguments(t *testing.T) {
	// Variable references in arguments
	fields := extractTopLevelFields(`{ user(id: $id) { name } posts(userId: $id, first: $count) { title } }`)
	assert.Equal(t, []string{"user", "posts"}, fields)
}

func TestExtractTopLevelFields_InlineFragment(t *testing.T) {
	// Inline fragment: ... on TypeName should not extract TypeName
	fields := extractTopLevelFields(`{ search { ... on User { id name } ... on Post { id title } } }`)
	assert.Equal(t, []string{"search"}, fields)
}

func TestExtractTopLevelFields_TopLevelInlineFragment(t *testing.T) {
	// Inline fragment at top level (depth 1)
	fields := extractTopLevelFields(`{ user { id } ... on Query { admin { id } } }`)
	assert.Equal(t, []string{"user"}, fields)
	assert.NotContains(t, fields, "Query")
	assert.NotContains(t, fields, "admin")
}

func TestExtractTopLevelFields_NamedFragmentSpread(t *testing.T) {
	// Named fragment spread at depth 1 — fragment name should not be a field
	fields := extractTopLevelFields(`{ user { id } ...AdminFields }`)
	assert.Equal(t, []string{"user"}, fields)
	assert.NotContains(t, fields, "AdminFields")
}

func TestExtractTopLevelFields_Aliases(t *testing.T) {
	// Aliases: both alias and field name appear as identifiers at depth 1.
	// Both are extracted (alias is response key, field name is resolver).
	// Arguments should NOT be extracted.
	fields := extractTopLevelFields(`{ smallPic: profilePicture(size: 64) { url } largePic: profilePicture(size: 256) { url } }`)
	assert.Contains(t, fields, "smallPic")
	assert.Contains(t, fields, "largePic")
	assert.Contains(t, fields, "profilePicture") // field name behind alias
	assert.NotContains(t, fields, "size")         // argument, not a field
	assert.NotContains(t, fields, "url")          // nested field, depth 2+
}

func TestExtractTopLevelFields_DirectivesSkipped(t *testing.T) {
	// Directives with arguments should not leak argument names
	fields := extractTopLevelFields(`{ user @include(if: $showUser) { id } posts { id } }`)
	assert.Contains(t, fields, "user")
	assert.Contains(t, fields, "posts")
	assert.NotContains(t, fields, "include")
}

func TestExtractTopLevelFields_CommentsSkipped(t *testing.T) {
	fields := extractTopLevelFields("{ # this is a comment\n  user { id }\n  # another comment\n  posts { id } }")
	assert.Equal(t, []string{"user", "posts"}, fields)
}

func TestParseRequestBody_ComplexQueryWithVariables(t *testing.T) {
	// Real-world style query with variables, arguments, and nested selections
	body := []byte(`{
		"query": "query SearchProducts($keyword: String!, $navParam: String!, $storeId: String!, $pageSize: Int!) { searchModel(keyword: $keyword, navParam: $navParam, storeId: $storeId) { products(first: $pageSize) { id title pricing { value } } searchReport { totalProducts } } }",
		"operationName": "SearchProducts",
		"variables": {"keyword": "", "navParam": "10000003", "storeId": "910", "pageSize": 24}
	}`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)

	op := result.Operations[0]
	assert.Equal(t, "SearchProducts", op.Name)
	assert.Equal(t, "query", op.Type)
	assert.Equal(t, []string{"searchModel"}, op.Fields)
	assert.True(t, op.HasVariables)
}

func TestParseRequestBody_ComplexMutationWithInputType(t *testing.T) {
	body := []byte(`{
		"query": "mutation CreateReview($input: ReviewInput!) { createReview(input: $input) { id rating text author { id name } } }",
		"operationName": "CreateReview",
		"variables": {"input": {"productId": "123", "rating": 5, "text": "Great"}}
	}`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)

	op := result.Operations[0]
	assert.Equal(t, "CreateReview", op.Name)
	assert.Equal(t, "mutation", op.Type)
	assert.Equal(t, []string{"createReview"}, op.Fields)
	assert.NotContains(t, op.Fields, "input")
}

func TestParseRequestBody_BatchedMixed(t *testing.T) {
	// Batched request mixing queries, mutations, and anonymous operations
	body := []byte(`[
		{"query": "query GetCart { cart { items { id } } }"},
		{"query": "mutation AddItem($id: ID!) { addToCart(itemId: $id) { success } }", "variables": {"id": "abc"}},
		{"query": "{ viewer { id } }"}
	]`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)
	assert.True(t, result.IsBatched)
	require.Len(t, result.Operations, 3)

	assert.Equal(t, "GetCart", result.Operations[0].Name)
	assert.Equal(t, "query", result.Operations[0].Type)
	assert.Equal(t, []string{"cart"}, result.Operations[0].Fields)

	assert.Equal(t, "AddItem", result.Operations[1].Name)
	assert.Equal(t, "mutation", result.Operations[1].Type)
	assert.Equal(t, []string{"addToCart"}, result.Operations[1].Fields)
	assert.NotContains(t, result.Operations[1].Fields, "itemId")

	assert.Equal(t, "anonymous", result.Operations[2].Name)
	assert.Equal(t, "query", result.Operations[2].Type)
	assert.Equal(t, []string{"viewer"}, result.Operations[2].Fields)
}

func TestParseRequestBody_QueryWithFragmentDefinition(t *testing.T) {
	// Query followed by fragment definition — only fields from the operation's
	// selection set should be extracted, not from the fragment body
	body := []byte(`{
		"query": "query GetUser { user { ...UserFields } } fragment UserFields on User { id name email avatar(size: 128) { url } }",
		"operationName": "GetUser"
	}`)

	result, err := ParseRequestBody(body)
	require.NoError(t, err)

	op := result.Operations[0]
	assert.Equal(t, "GetUser", op.Name)
	assert.Equal(t, []string{"user"}, op.Fields)
}

func TestParseRequestBody_WhitespaceBody(t *testing.T) {
	_, err := ParseRequestBody([]byte("   \n\t  "))
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrEmpty))
	assert.True(t, IsNotGraphQL(err))
}

// ---------------------------------------------------------------------------
// Real-world complex GraphQL queries
// ---------------------------------------------------------------------------

func TestParseRequestBody_RealWorld_NavWithInlineFragments(t *testing.T) {
	// Complex query with deeply nested inline fragments across multiple union types
	query := `query navigation($id: String!, $section: NavSection!) {
    navigation(id: $id, section: $section) {
        ... on TopLevelMenuNode {
            id
            nodeType
            label
            description
            href
            iconRef
            thumbnail {
                sourceSelector {
                    mediaInfo {
                        resolvedUrl
                        __typename
                    }
                    __typename
                }
                fallbackContent {
                    url
                    __typename
                }
                __typename
            }
            children {
                id
                nodeType
                menuItems {
                    id
                    nodeType
                    label
                    description
                    href
                    iconRef
                    thumbnail {
                        sourceSelector {
                            mediaInfo {
                                resolvedUrl
                                __typename
                            }
                            __typename
                        }
                        fallbackContent {
                            url
                            __typename
                        }
                        __typename
                    }
                    children {
                        id
                        nodeType
                        __typename
                    }
                    __typename
                }
                __typename
            }
            __typename
        }
        ... on SecondaryMenuNode {
            id
            nodeType
            label
            __typename
        }
        ... on UtilityMenuNode {
            id
            nodeType
            __typename
        }
        ... on FooterMenuNode {
            id
            nodeType
            __typename
        }
        ... on SidebarMenuNode {
            id
            nodeType
            __typename
        }
        __typename
    }
}`

	result, err := ParseRequestBody([]byte(`{"query": ` + jsonEscapeQuery(query) + `, "operationName": "navigation", "variables": {"id": "abc", "section": "MAIN_NAV"}}`))
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)

	op := result.Operations[0]
	assert.Equal(t, "navigation", op.Name)
	assert.Equal(t, "query", op.Type)
	assert.Equal(t, []string{"navigation"}, op.Fields)
	assert.True(t, op.HasVariables)

	// Ensure type names from inline fragments don't leak into fields
	for _, typeName := range []string{"TopLevelMenuNode", "SecondaryMenuNode", "UtilityMenuNode", "FooterMenuNode", "SidebarMenuNode"} {
		assert.NotContains(t, op.Fields, typeName)
	}
	// Ensure nested field names don't leak
	assert.NotContains(t, op.Fields, "nodeType")
	assert.NotContains(t, op.Fields, "thumbnail")
	assert.NotContains(t, op.Fields, "children")
}

func TestParseRequestBody_RealWorld_PromoSectionWithDirectives(t *testing.T) {
	// Complex query with @skip directive, many variables with defaults,
	// and deeply nested fields with arguments at multiple levels
	query := `query promoSection(
    $skipAnalytics: Boolean = false
    $locationId: String
    $enforcePricingPolicy: Boolean
    $filterKey: String
    $rankAlgorithm: String
    $limit: Int
    $originUrl: String!
    $userId: String
    $accountType: AccountType = STANDARD
    $groupId: String
    $enableVariant: Boolean
) {
    promoSection(
        originUrl: $originUrl
        locationId: $locationId
        userId: $userId
        accountType: $accountType
        groupId: $groupId
        enableVariant: $enableVariant
    ) {
        campaignInfo @skip(if: $skipAnalytics) {
            sourceType
            heading
            startDate
            endDate
            heroImage
            mobileHeroImage
            ctaLabel
            ctaUrl
            countdownDuration
            gradientColors
            badgeIcon
            segments {
                segment
                filterValue
                coverImage
                url
                landingUrl
                excluded
                showOnMobile
                __typename
            }
            attributeName
            sectionTitle
            iconUrl
            sectionSubtitle
            countdown {
                urgencyThreshold
                remainingDaysThreshold
                displayThreshold
                label
                __typename
            }
            __typename
        }
        featuredItems(filterKey: $filterKey, rankAlgorithm: $rankAlgorithm, limit: $limit) {
            coverImage
            groupName
            groupUrl
            filterValue
            items {
                sku
                dataSources
                tags(locationId: $locationId) {
                    label
                    __typename
                }
                metadata {
                    isFeatured
                    hideMsrp
                    __typename
                }
                details {
                    slug
                    itemType
                    displayName
                    sku
                    vendorName
                    __typename
                }
                assets {
                    photos {
                        src
                        kind
                        variant
                        dimensions
                        __typename
                    }
                    __typename
                }
                cost(locationId: $locationId, enforcePricingPolicy: $enforcePricingPolicy) {
                    listPrice
                    amount
                    __typename
                }
                ratings {
                    summary {
                        average
                        count
                        __typename
                    }
                    __typename
                }
                __typename
            }
            __typename
        }
        userPicksSection {
            coverImage
            groupName
            items {
                sku
                cost(locationId: $locationId) {
                    amount
                    __typename
                }
                __typename
            }
            __typename
        }
        __typename
    }
}`

	result, err := ParseRequestBody([]byte(`{"query": ` + jsonEscapeQuery(query) + `, "operationName": "promoSection", "variables": {"originUrl": "https://example.com", "locationId": "42"}}`))
	require.NoError(t, err)
	require.Len(t, result.Operations, 1)

	op := result.Operations[0]
	assert.Equal(t, "promoSection", op.Name)
	assert.Equal(t, "query", op.Type)
	assert.Equal(t, []string{"promoSection"}, op.Fields)
	assert.True(t, op.HasVariables)

	// Ensure directive names don't leak into fields
	assert.NotContains(t, op.Fields, "skip")
	assert.NotContains(t, op.Fields, "include")
	// Ensure argument names don't leak
	assert.NotContains(t, op.Fields, "originUrl")
	assert.NotContains(t, op.Fields, "locationId")
	assert.NotContains(t, op.Fields, "filterKey")
	// Ensure nested fields don't leak
	assert.NotContains(t, op.Fields, "campaignInfo")
	assert.NotContains(t, op.Fields, "featuredItems")
	assert.NotContains(t, op.Fields, "userPicksSection")
	assert.NotContains(t, op.Fields, "items")
}

// jsonEscapeQuery JSON-encodes a raw query string for embedding in a JSON body.
func jsonEscapeQuery(q string) string {
	b, _ := json.Marshal(q)
	return string(b)
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
