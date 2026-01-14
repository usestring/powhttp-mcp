package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"github.com/usestring/powhttp-mcp/pkg/client"
)

// refreshStrategy indicates how to handle a session refresh.
type refreshStrategy int

const (
	strategyAppendOnly refreshStrategy = iota
	strategyRebuild
)

// singleflight group for deduplicating concurrent refresh requests.
var refreshGroup singleflight.Group

// RefreshSession performs incremental or full refresh of a session's entries.
// Uses singleflight to deduplicate concurrent refresh requests for the same session.
// Applies a context timeout to prevent indefinite hangs.
func (idx *Indexer) RefreshSession(ctx context.Context, sessionID string) error {
	// Apply refresh timeout to prevent indefinite hangs
	refreshCtx, cancel := context.WithTimeout(ctx, idx.config.RefreshTimeout)
	defer cancel()

	_, err, _ := refreshGroup.Do(sessionID, func() (any, error) {
		return nil, idx.doRefresh(refreshCtx, sessionID)
	})
	return err
}

// RefreshIfStale checks freshness threshold and refreshes if needed.
func (idx *Indexer) RefreshIfStale(ctx context.Context, sessionID string) error {
	state := idx.getSessionStateCopy(sessionID)

	// Always refresh if never synced
	if state == nil || state.lastSyncAt.IsZero() {
		return idx.RefreshSession(ctx, sessionID)
	}

	// Check if stale
	if time.Since(state.lastSyncAt) > idx.config.FreshnessThreshold {
		return idx.RefreshSession(ctx, sessionID)
	}

	return nil
}

// StartBackgroundRefresh starts a goroutine that periodically refreshes all sessions.
func (idx *Indexer) StartBackgroundRefresh(ctx context.Context) {
	slog.Info("starting background refresh for all sessions",
		slog.Duration("interval", idx.config.RefreshInterval),
	)

	go func() {
		ticker := time.NewTicker(idx.config.RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("stopping background refresh")
				return
			case <-ticker.C:
				idx.refreshAllSessions(ctx)
			}
		}
	}()
}

// refreshAllSessions lists and refreshes all available sessions.
func (idx *Indexer) refreshAllSessions(ctx context.Context) {
	sessions, err := idx.client.ListSessions(ctx)
	if err != nil {
		slog.Warn("failed to list sessions for background refresh",
			slog.String("error", err.Error()),
		)
		return
	}

	for _, session := range sessions {
		if err := idx.RefreshSession(ctx, session.ID); err != nil {
			slog.Warn("background refresh failed",
				slog.String("session_id", session.ID),
				slog.String("error", err.Error()),
			)
		}
	}
}

// doRefresh performs the actual refresh logic.
func (idx *Indexer) doRefresh(ctx context.Context, sessionID string) error {
	start := time.Now()

	// Fetch current session to get entry IDs
	session, err := idx.client.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("fetching session: %w", err)
	}

	currentEntryIDs := session.EntryIDs
	state := idx.getSessionStateCopy(sessionID)

	// Determine refresh strategy
	strategy := idx.detectRefreshStrategy(currentEntryIDs, state)
	strategyName := "append_only"
	if strategy == strategyRebuild {
		strategyName = "rebuild"
	}

	var entriesToFetch []string

	switch strategy {
	case strategyAppendOnly:
		// Only fetch new entries
		if state != nil && state.lastEntryIDsLen < len(currentEntryIDs) {
			entriesToFetch = currentEntryIDs[state.lastEntryIDsLen:]
		}
	case strategyRebuild:
		// Fetch last BOOTSTRAP_TAIL_LIMIT entries
		limit := idx.config.BootstrapTailLimit
		if len(currentEntryIDs) <= limit {
			entriesToFetch = currentEntryIDs
		} else {
			entriesToFetch = currentEntryIDs[len(currentEntryIDs)-limit:]
		}
	}

	if len(entriesToFetch) == 0 {
		// Update sync time even if nothing to fetch
		idx.updateSessionState(sessionID, currentEntryIDs)
		slog.Debug("refresh completed with no new entries",
			slog.String("session_id", sessionID),
			slog.String("strategy", strategyName),
			slog.Int("total_entries", len(currentEntryIDs)),
		)
		return nil
	}

	// Fetch entries concurrently
	entries, err := idx.fetchEntriesConcurrently(ctx, sessionID, entriesToFetch)
	if err != nil {
		return fmt.Errorf("fetching entries: %w", err)
	}

	// Index all fetched entries
	indexed := 0
	for _, entry := range entries {
		if entry != nil {
			idx.Index(entry)
			indexed++
		}
	}

	// Update session state
	idx.updateSessionState(sessionID, currentEntryIDs)

	slog.Info("refresh completed",
		slog.String("session_id", sessionID),
		slog.String("strategy", strategyName),
		slog.Int("fetched", len(entriesToFetch)),
		slog.Int("indexed", indexed),
		slog.Int("total_entries", len(currentEntryIDs)),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
	)

	return nil
}

// detectRefreshStrategy determines append-only vs rebuild based on entry ID comparison.
func (idx *Indexer) detectRefreshStrategy(currentEntryIDs []string, state *sessionState) refreshStrategy {
	// First sync always rebuilds
	if state == nil || state.lastSyncAt.IsZero() {
		return strategyRebuild
	}

	// If length shrank, something was deleted - rebuild
	if len(currentEntryIDs) < state.lastEntryIDsLen {
		return strategyRebuild
	}

	// If previous tail position no longer matches, entries were modified - rebuild
	if state.lastEntryIDsLen > 0 && len(currentEntryIDs) >= state.lastEntryIDsLen {
		prevTailIdx := state.lastEntryIDsLen - 1
		if currentEntryIDs[prevTailIdx] != state.lastTailEntryID {
			return strategyRebuild
		}
	}

	// Length grew and tail matches - safe to append only
	return strategyAppendOnly
}

// fetchEntriesConcurrently fetches entries using a worker pool.
func (idx *Indexer) fetchEntriesConcurrently(ctx context.Context, sessionID string, entryIDs []string) ([]*client.SessionEntry, error) {
	entries := make([]*client.SessionEntry, len(entryIDs))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(idx.config.FetchWorkers)

	for i, entryID := range entryIDs {
		i, entryID := i, entryID // Capture loop variables

		g.Go(func() error {
			// Check cache first
			if idx.cache != nil {
				if cached, ok := idx.cache.Get(entryID); ok {
					entries[i] = cached
					return nil
				}
			}

			// Fetch from API
			entry, err := idx.client.GetEntry(ctx, sessionID, entryID)
			if err != nil {
				// Don't fail the whole batch for one entry
				// Could be a race condition where entry was deleted
				slog.Debug("failed to fetch entry",
					slog.String("session_id", sessionID),
					slog.String("entry_id", entryID),
					slog.String("error", err.Error()),
				)
				return nil
			}

			entries[i] = entry
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return entries, nil
}

// LastSyncTime returns the last sync time for a session.
func (idx *Indexer) LastSyncTime(sessionID string) time.Time {
	state := idx.getSessionStateCopy(sessionID)
	if state == nil {
		return time.Time{}
	}
	return state.lastSyncAt
}
