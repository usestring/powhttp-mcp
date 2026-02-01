package indexer

import (
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/RoaringBitmap/roaring/v2"

	"github.com/usestring/powhttp-mcp/internal/cache"
	"github.com/usestring/powhttp-mcp/internal/config"
	"github.com/usestring/powhttp-mcp/pkg/client"
)

// sessionState tracks refresh state for a single session.
type sessionState struct {
	lastEntryIDsLen int
	lastTailEntryID string
	lastSyncAt      time.Time
}

// Indexer maintains in-memory indexes over HTTP entries using Roaring bitmaps.
type Indexer struct {
	mu sync.RWMutex

	// ID mappings
	idToDoc   map[string]uint32
	docToMeta []*EntryMeta
	nextDocID uint32

	// Inverted indexes (all use Roaring bitmaps)
	idxHost          map[string]*roaring.Bitmap
	idxMethod        map[string]*roaring.Bitmap
	idxProcessName   map[string]*roaring.Bitmap
	idxPID           map[int]*roaring.Bitmap
	idxStatus        map[int]*roaring.Bitmap
	idxHTTPVersion   map[string]*roaring.Bitmap
	idxHeaderName    map[string]*roaring.Bitmap
	idxHeaderValue   map[string]*roaring.Bitmap // key format: "header-name:header-value"
	idxTLSConnection map[string]*roaring.Bitmap
	idxH2Connection  map[string]*roaring.Bitmap
	idxJA3           map[string]*roaring.Bitmap
	idxJA4           map[string]*roaring.Bitmap
	idxToken         map[string]*roaring.Bitmap
	idxHeaderToken   map[string]*roaring.Bitmap
	idxBodyToken     map[string]*roaring.Bitmap

	// Per-session refresh state
	sessions map[string]*sessionState

	// Dependencies
	client *client.Client
	cache  *cache.EntryCache
	config *config.Config
}

// New creates a new Indexer instance.
func New(c *client.Client, cache *cache.EntryCache, cfg *config.Config) *Indexer {
	return &Indexer{
		idToDoc:          make(map[string]uint32),
		docToMeta:        make([]*EntryMeta, 0, 1024),
		idxHost:          make(map[string]*roaring.Bitmap),
		idxMethod:        make(map[string]*roaring.Bitmap),
		idxProcessName:   make(map[string]*roaring.Bitmap),
		idxPID:           make(map[int]*roaring.Bitmap),
		idxStatus:        make(map[int]*roaring.Bitmap),
		idxHTTPVersion:   make(map[string]*roaring.Bitmap),
		idxHeaderName:    make(map[string]*roaring.Bitmap),
		idxHeaderValue:   make(map[string]*roaring.Bitmap),
		idxTLSConnection: make(map[string]*roaring.Bitmap),
		idxH2Connection:  make(map[string]*roaring.Bitmap),
		idxJA3:           make(map[string]*roaring.Bitmap),
		idxJA4:           make(map[string]*roaring.Bitmap),
		idxToken:         make(map[string]*roaring.Bitmap),
		idxHeaderToken:   make(map[string]*roaring.Bitmap),
		idxBodyToken:     make(map[string]*roaring.Bitmap),
		sessions:         make(map[string]*sessionState),
		client:           c,
		cache:            cache,
		config:           cfg,
	}
}

// Index adds or updates an entry in the index.
// Returns the assigned document ID.
func (idx *Indexer) Index(entry *client.SessionEntry) uint32 {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Check if already indexed
	if docID, exists := idx.idToDoc[entry.ID]; exists {
		return docID
	}

	// Assign new document ID
	docID := idx.nextDocID
	idx.nextDocID++

	// Convert to metadata
	meta := FromSessionEntry(entry)
	meta.DocID = docID

	// Store mappings
	idx.idToDoc[entry.ID] = docID
	idx.docToMeta = append(idx.docToMeta, meta)

	// Index by host
	if meta.Host != "" {
		idx.addToBitmap(idx.idxHost, meta.Host, docID)
	}

	// Index by method
	if meta.Method != "" {
		idx.addToBitmap(idx.idxMethod, meta.Method, docID)
	}

	// Index by process name
	if meta.ProcessName != "" {
		idx.addToBitmap(idx.idxProcessName, meta.ProcessName, docID)
	}

	// Index by PID
	if meta.PID != 0 {
		idx.addToIntBitmap(idx.idxPID, meta.PID, docID)
	}

	// Index by status
	if meta.Status != 0 {
		idx.addToIntBitmap(idx.idxStatus, meta.Status, docID)
	}

	// Index by HTTP version
	if meta.HTTPVersion != "" {
		idx.addToBitmap(idx.idxHTTPVersion, meta.HTTPVersion, docID)
	}

	// Index by header names
	for _, header := range meta.HeaderNamesLower {
		idx.addToBitmap(idx.idxHeaderName, header, docID)
	}

	// Index by header name:value pairs
	for _, hv := range meta.HeaderValues {
		key := hv.Name + ":" + hv.Value
		idx.addToBitmap(idx.idxHeaderValue, key, docID)
	}

	// Index by TLS connection ID
	if meta.TLSConnectionID != "" {
		idx.addToBitmap(idx.idxTLSConnection, meta.TLSConnectionID, docID)
	}

	// Index by HTTP/2 connection ID
	if meta.H2ConnectionID != "" {
		idx.addToBitmap(idx.idxH2Connection, meta.H2ConnectionID, docID)
	}

	// Index by JA3
	if meta.JA3 != "" {
		idx.addToBitmap(idx.idxJA3, meta.JA3, docID)
	}

	// Index by JA4
	if meta.JA4 != "" {
		idx.addToBitmap(idx.idxJA4, meta.JA4, docID)
	}

	// Index URL tokens
	tokens := TokenizeURL(meta.URL)
	for _, token := range tokens {
		idx.addToBitmap(idx.idxToken, token, docID)
	}

	// Index header tokens (full header field: "name: value")
	headerTokens := TokenizeHeaders(meta.HeaderValues)
	for _, token := range headerTokens {
		idx.addToBitmap(idx.idxHeaderToken, token, docID)
	}

	// Index body tokens (when enabled via config)
	if idx.config != nil && idx.config.IndexBody {
		idx.indexBodyTokens(entry, meta, docID)
	}

	// Cache the full entry
	if idx.cache != nil {
		idx.cache.Put(entry.ID, entry)
	}

	return docID
}

// indexBodyTokens decodes and tokenizes request and response bodies.
func (idx *Indexer) indexBodyTokens(entry *client.SessionEntry, meta *EntryMeta, docID uint32) {
	maxBytes := 65536
	if idx.config.IndexBodyMaxBytes > 0 {
		maxBytes = idx.config.IndexBodyMaxBytes
	}

	// Tokenize request body
	if entry.Request.Body != nil && *entry.Request.Body != "" {
		reqContentType := ""
		for _, pair := range entry.Request.Headers {
			if len(pair) >= 2 && pair[0] == "content-type" {
				reqContentType = pair[1]
				break
			}
		}
		if bodyBytes, err := base64.StdEncoding.DecodeString(*entry.Request.Body); err == nil {
			for _, token := range TokenizeBody(reqContentType, bodyBytes, maxBytes) {
				idx.addToBitmap(idx.idxBodyToken, token, docID)
			}
		}
	}

	// Tokenize response body
	if entry.Response != nil && entry.Response.Body != nil && *entry.Response.Body != "" {
		if bodyBytes, err := base64.StdEncoding.DecodeString(*entry.Response.Body); err == nil {
			for _, token := range TokenizeBody(meta.RespContentType, bodyBytes, maxBytes) {
				idx.addToBitmap(idx.idxBodyToken, token, docID)
			}
		}
	}
}

// GetMeta retrieves metadata by docID.
func (idx *Indexer) GetMeta(docID uint32) *EntryMeta {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if int(docID) >= len(idx.docToMeta) {
		return nil
	}
	return idx.docToMeta[docID]
}

// GetMetaByEntryID retrieves metadata by entry ID.
func (idx *Indexer) GetMetaByEntryID(entryID string) *EntryMeta {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	docID, exists := idx.idToDoc[entryID]
	if !exists {
		return nil
	}
	if int(docID) >= len(idx.docToMeta) {
		return nil
	}
	return idx.docToMeta[docID]
}

// AllDocIDs returns a bitmap of all indexed document IDs.
func (idx *Indexer) AllDocIDs() *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	bm := roaring.New()
	for i := uint32(0); i < idx.nextDocID; i++ {
		bm.Add(i)
	}
	return bm
}

// DocCount returns the number of indexed documents.
func (idx *Indexer) DocCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.docToMeta)
}

// GetBitmapForHost returns the bitmap for a host pattern.
// Supports wildcard prefix: "*.example.com" matches "example.com"
// and all subdomains like "api.example.com", "www.example.com".
// Without the prefix, matches exactly.
func (idx *Indexer) GetBitmapForHost(host string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if !strings.HasPrefix(host, "*.") {
		return idx.idxHost[host]
	}

	baseDomain := host[2:] // safe: HasPrefix guarantees len >= 2
	if baseDomain == "" {  // handles "*." edge case
		return nil
	}

	suffix := "." + baseDomain
	result := roaring.New()
	for key, bm := range idx.idxHost {
		if key == baseDomain || strings.HasSuffix(key, suffix) {
			result.Or(bm)
		}
	}
	if result.IsEmpty() {
		return nil
	}
	return result
}

// GetBitmapForMethod returns the bitmap for a specific HTTP method.
func (idx *Indexer) GetBitmapForMethod(method string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxMethod[method]
}

// GetBitmapForProcessName returns the bitmap for a specific process name.
func (idx *Indexer) GetBitmapForProcessName(name string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxProcessName[name]
}

// GetBitmapForPID returns the bitmap for a specific PID.
func (idx *Indexer) GetBitmapForPID(pid int) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxPID[pid]
}

// GetBitmapForStatus returns the bitmap for a specific HTTP status code.
func (idx *Indexer) GetBitmapForStatus(status int) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxStatus[status]
}

// GetBitmapForHTTPVersion returns the bitmap for a specific HTTP version.
func (idx *Indexer) GetBitmapForHTTPVersion(version string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxHTTPVersion[version]
}

// GetBitmapForHeaderName returns the bitmap for a specific header name.
func (idx *Indexer) GetBitmapForHeaderName(name string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxHeaderName[name]
}

// GetBitmapForHeaderValue returns the bitmap for a specific header name:value pair.
func (idx *Indexer) GetBitmapForHeaderValue(name, value string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	key := name + ":" + value
	return idx.idxHeaderValue[key]
}

// GetBitmapForTLSConnection returns the bitmap for a specific TLS connection ID.
func (idx *Indexer) GetBitmapForTLSConnection(connID string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxTLSConnection[connID]
}

// GetBitmapForH2Connection returns the bitmap for a specific HTTP/2 connection ID.
func (idx *Indexer) GetBitmapForH2Connection(connID string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxH2Connection[connID]
}

// GetBitmapForJA3 returns the bitmap for a specific JA3 fingerprint.
func (idx *Indexer) GetBitmapForJA3(ja3 string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxJA3[ja3]
}

// GetBitmapForJA4 returns the bitmap for a specific JA4 fingerprint.
func (idx *Indexer) GetBitmapForJA4(ja4 string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxJA4[ja4]
}

// GetBitmapForToken returns the bitmap for a specific URL token.
func (idx *Indexer) GetBitmapForToken(token string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxToken[token]
}

// GetBitmapForHeaderToken returns the bitmap for a specific header token.
func (idx *Indexer) GetBitmapForHeaderToken(token string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxHeaderToken[token]
}

// GetBitmapForBodyToken returns the bitmap for a specific body token.
func (idx *Indexer) GetBitmapForBodyToken(token string) *roaring.Bitmap {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.idxBodyToken[token]
}

// BodyIndexEnabled returns whether body indexing is enabled.
func (idx *Indexer) BodyIndexEnabled() bool {
	return idx.config != nil && idx.config.IndexBody
}

// addToBitmap adds a docID to a string-keyed bitmap index.
func (idx *Indexer) addToBitmap(index map[string]*roaring.Bitmap, key string, docID uint32) {
	bm, exists := index[key]
	if !exists {
		bm = roaring.New()
		index[key] = bm
	}
	bm.Add(docID)
}

// addToIntBitmap adds a docID to an int-keyed bitmap index.
func (idx *Indexer) addToIntBitmap(index map[int]*roaring.Bitmap, key int, docID uint32) {
	bm, exists := index[key]
	if !exists {
		bm = roaring.New()
		index[key] = bm
	}
	bm.Add(docID)
}

// getSessionState returns or creates session state for tracking refresh.
func (idx *Indexer) getSessionState(sessionID string) *sessionState {
	state, exists := idx.sessions[sessionID]
	if !exists {
		state = &sessionState{}
		idx.sessions[sessionID] = state
	}
	return state
}

// updateSessionState updates the session state after a refresh.
func (idx *Indexer) updateSessionState(sessionID string, entryIDs []string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	state := idx.getSessionState(sessionID)
	state.lastEntryIDsLen = len(entryIDs)
	if len(entryIDs) > 0 {
		state.lastTailEntryID = entryIDs[len(entryIDs)-1]
	} else {
		state.lastTailEntryID = ""
	}
	state.lastSyncAt = time.Now()
}

// getSessionStateCopy returns a copy of the session state for reading.
func (idx *Indexer) getSessionStateCopy(sessionID string) *sessionState {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	state, exists := idx.sessions[sessionID]
	if !exists {
		return nil
	}
	return &sessionState{
		lastEntryIDsLen: state.lastEntryIDsLen,
		lastTailEntryID: state.lastTailEntryID,
		lastSyncAt:      state.lastSyncAt,
	}
}

// Client returns the underlying powhttp client.
func (idx *Indexer) Client() *client.Client {
	return idx.client
}

// Config returns the configuration.
func (idx *Indexer) Config() *config.Config {
	return idx.config
}
