package tools

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// SessionsListInput is the input for powhttp_sessions_list.
type SessionsListInput struct{}

// SessionsListOutput is the output for powhttp_sessions_list.
type SessionsListOutput struct {
	Sessions []SessionInfo `json:"sessions"`
}

// SessionInfo is a summary of a session.
type SessionInfo struct {
	SessionID  string `json:"session_id"`
	Name       string `json:"name"`
	EntryCount int    `json:"entry_count"`
}

// SessionActiveInput is the input for powhttp_session_active.
type SessionActiveInput struct{}

// SessionActiveOutput is the output for powhttp_session_active.
type SessionActiveOutput struct {
	Session *SessionInfo `json:"session"`
}

// ToolSessionsList lists all sessions.
func ToolSessionsList(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input SessionsListInput) (*sdkmcp.CallToolResult, SessionsListOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input SessionsListInput) (*sdkmcp.CallToolResult, SessionsListOutput, error) {
		sessions, err := d.Client.ListSessions(ctx)
		if err != nil {
			return nil, SessionsListOutput{}, WrapPowHTTPError(err)
		}

		output := SessionsListOutput{
			Sessions: make([]SessionInfo, len(sessions)),
		}
		for i, sess := range sessions {
			output.Sessions[i] = SessionInfo{
				SessionID:  sess.ID,
				Name:       sess.Name,
				EntryCount: len(sess.EntryIDs),
			}
		}

		return nil, output, nil
	}
}

// ToolSessionActive gets the active session.
func ToolSessionActive(d *Deps) func(ctx context.Context, req *sdkmcp.CallToolRequest, input SessionActiveInput) (*sdkmcp.CallToolResult, SessionActiveOutput, error) {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest, input SessionActiveInput) (*sdkmcp.CallToolResult, SessionActiveOutput, error) {
		session, err := d.Client.GetSession(ctx, "active")
		if err != nil {
			return nil, SessionActiveOutput{}, WrapPowHTTPError(err)
		}

		return nil, SessionActiveOutput{
			Session: &SessionInfo{
				SessionID:  session.ID,
				Name:       session.Name,
				EntryCount: len(session.EntryIDs),
			},
		}, nil
	}
}
