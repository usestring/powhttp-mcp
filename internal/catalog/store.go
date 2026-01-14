// Package catalog provides endpoint cataloging and clustering functionality.
package catalog

import (
	"sync"
	"time"

	"github.com/usestring/powhttp-mcp/pkg/types"
)

// ClusterStore holds computed clusters for reference by describe/flow tools.
type ClusterStore struct {
	mu       sync.RWMutex
	clusters map[string]*StoredCluster        // keyed by cluster_id
	scopes   map[string]*types.ExtractResponse // keyed by scope_hash
}

// StoredCluster holds a cluster with all its entry IDs.
type StoredCluster struct {
	Cluster   *types.Cluster
	EntryIDs  []string // All entry IDs, not just examples
	CreatedAt time.Time
}

// NewClusterStore creates a new ClusterStore.
func NewClusterStore() *ClusterStore {
	return &ClusterStore{
		clusters: make(map[string]*StoredCluster),
		scopes:   make(map[string]*types.ExtractResponse),
	}
}

// StoreExtraction stores an extraction result with all entry IDs.
// fullEntryIDs is a map from cluster_id to all entry IDs in that cluster.
func (s *ClusterStore) StoreExtraction(resp *types.ExtractResponse, fullEntryIDs map[string][]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Store each cluster with its full entry IDs
	for i := range resp.Clusters {
		cluster := &resp.Clusters[i]
		s.clusters[cluster.ID] = &StoredCluster{
			Cluster:   cluster,
			EntryIDs:  fullEntryIDs[cluster.ID],
			CreatedAt: now,
		}
	}

	// Store the scope mapping
	s.scopes[resp.ScopeHash] = resp
}

// GetCluster retrieves a single cluster by ID.
func (s *ClusterStore) GetCluster(clusterID string) (*StoredCluster, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cluster, ok := s.clusters[clusterID]
	return cluster, ok
}

// GetScope retrieves an extraction by scope hash.
func (s *ClusterStore) GetScope(scopeHash string) (*types.ExtractResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp, ok := s.scopes[scopeHash]
	return resp, ok
}
