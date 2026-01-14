package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Highlight colors for ListEntriesOptions.Highlighted
const (
	HighlightRed           = "red"
	HighlightGreen         = "green"
	HighlightBlue          = "blue"
	HighlightYellow        = "yellow"
	HighlightGray          = "gray"
	HighlightOrange        = "orange"
	HighlightPink          = "pink"
	HighlightPurple        = "purple"
	HighlightStrikethrough = "strikethrough"
)

// WebSocket message types
const (
	WSMessageText    = "text"
	WSMessageBinary  = "binary"
	WSMessageClose   = "close"
	WSMessagePing    = "ping"
	WSMessagePong    = "pong"
	WSMessageUnknown = "unknown"
)

// WebSocket sides
const (
	WSSideClient = "client"
	WSSideServer = "server"
)

// Transaction types
const (
	TransactionRequest     = "request"
	TransactionPushPromise = "push_promise"
)

// Session represents a powhttp session containing captured network traffic.
type Session struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	EntryIDs []string `json:"entryIds"`
}

// SocketAddress represents an IP address and optional port.
type SocketAddress struct {
	IP   string `json:"ip"`
	Port *int   `json:"port"`
}

// Headers is a slice of header key-value pairs.
type Headers [][]string

// Get returns the first value for the given header name (case-insensitive).
// Returns an empty string if the header is not found.
func (h Headers) Get(name string) string {
	name = strings.ToLower(name)
	for _, pair := range h {
		if len(pair) >= 2 && strings.ToLower(pair[0]) == name {
			return pair[1]
		}
	}
	return ""
}

// Values returns all values for the given header name (case-insensitive).
func (h Headers) Values(name string) []string {
	name = strings.ToLower(name)
	var values []string
	for _, pair := range h {
		if len(pair) >= 2 && strings.ToLower(pair[0]) == name {
			values = append(values, pair[1])
		}
	}
	return values
}

// Request represents an HTTP request.
type Request struct {
	Method      *string `json:"method"`
	Path        *string `json:"path"`
	HTTPVersion *string `json:"httpVersion"`
	Headers     Headers `json:"headers"`
	Body        *string `json:"body"` // Base64-encoded
}

// Response represents an HTTP response.
type Response struct {
	HTTPVersion *string `json:"httpVersion"`
	StatusCode  *int    `json:"statusCode"`
	StatusText  *string `json:"statusText"`
	Headers     Headers `json:"headers"`
	Body        *string `json:"body"` // Base64-encoded
}

// JA3Fingerprint represents a JA3 TLS fingerprint.
type JA3Fingerprint struct {
	String string `json:"string"`
	Hash   string `json:"hash"`
}

// JA4Fingerprint represents a JA4 TLS fingerprint.
type JA4Fingerprint struct {
	Raw    string `json:"raw"`
	Hashed string `json:"hashed"`
}

// TLSInfo contains TLS connection information for an entry.
type TLSInfo struct {
	ConnectionID *string         `json:"connectionId"`
	TLSVersion   *int            `json:"tlsVersion"`
	CipherSuite  *int            `json:"cipherSuite"`
	JA3          *JA3Fingerprint `json:"ja3"`
	JA4          *JA4Fingerprint `json:"ja4"`
}

// HTTP2Info contains HTTP/2 connection information for an entry.
type HTTP2Info struct {
	ConnectionID string `json:"connectionId"`
	StreamID     int    `json:"streamId"`
}

// Timings contains timing information for an HTTP transaction.
type Timings struct {
	StartedAt int64  `json:"startedAt"` // Unix timestamp in milliseconds
	Blocked   *int64 `json:"blocked"`
	DNS       *int64 `json:"dns"`
	Connect   *int64 `json:"connect"`
	Send      *int64 `json:"send"`
	Wait      *int64 `json:"wait"`
	Receive   *int64 `json:"receive"`
	SSL       *int64 `json:"ssl"`
}

// ProcessInfo contains information about the process that made the request.
type ProcessInfo struct {
	PID  int     `json:"pid"`
	Name *string `json:"name"`
}

// SessionEntry represents an individual HTTP transaction captured within a session.
type SessionEntry struct {
	ID              string         `json:"id"`
	URL             string         `json:"url"`
	ClientAddr      *SocketAddress `json:"clientAddr"`
	RemoteAddr      *SocketAddress `json:"remoteAddr"`
	HTTPVersion     string         `json:"httpVersion"`
	TransactionType string         `json:"transactionType"` // "request" or "push_promise"
	Request         Request        `json:"request"`
	Response        *Response      `json:"response"`
	IsWebSocket     bool           `json:"isWebSocket"`
	TLS             TLSInfo        `json:"tls"`
	HTTP2           *HTTP2Info     `json:"http2"`
	Timings         Timings        `json:"timings"`
	Process         *ProcessInfo   `json:"process"`
}

// WebSocketContent represents the content of a WebSocket message.
// Check the Type field to determine which content fields are populated.
type WebSocketContent struct {
	Type   string `json:"type"`             // "text", "binary", "close", "ping", "pong", "unknown"
	Text   string `json:"text,omitempty"`   // when Type == "text"
	Data   string `json:"data,omitempty"`   // when Type == "binary"/"ping"/"pong"/"unknown" (base64)
	Code   int    `json:"code,omitempty"`   // when Type == "close"
	Reason string `json:"reason,omitempty"` // when Type == "close" (base64)
}

// WebSocketMessage represents a single WebSocket message.
type WebSocketMessage struct {
	Side      string           `json:"side"` // "client" or "server"
	StartedAt *int64           `json:"startedAt"`
	EndedAt   *int64           `json:"endedAt"`
	Content   WebSocketContent `json:"content"`
}

// TLS event sides
const (
	TLSSideClient = "client"
	TLSSideServer = "server"
)

// TLS message types
const (
	TLSMsgHandshake        = "handshake"
	TLSMsgChangeCipherSpec = "change_cipher_spec"
)

// TLS handshake message types
const (
	TLSHandshakeClientHello         = "client_hello"
	TLSHandshakeServerHello         = "server_hello"
	TLSHandshakeEncryptedExtensions = "encrypted_extensions"
	TLSHandshakeCertificate         = "certificate"
	TLSHandshakeCertificateVerify   = "certificate_verify"
	TLSHandshakeFinished            = "finished"
)

// TLSNamedValue represents a value with its numeric code and human-readable name.
type TLSNamedValue struct {
	Value int    `json:"value"`
	Name  string `json:"name"`
}

// TLSExtension represents a TLS extension with variable content.
type TLSExtension struct {
	Value int             `json:"value"`
	Name  string          `json:"name"`
	Data  json.RawMessage `json:"-"` // Remaining fields vary by extension type
}

// TLSClientHello represents a TLS ClientHello message.
type TLSClientHello struct {
	Version            TLSNamedValue   `json:"version"`
	Random             string          `json:"random"`
	SessionID          string          `json:"session_id"`
	CipherSuites       []TLSNamedValue `json:"cipher_suites"`
	CompressionMethods []TLSNamedValue `json:"compression_methods"`
	Extensions         json.RawMessage `json:"extensions"` // Complex, varies by extension type
}

// TLSServerHello represents a TLS ServerHello message.
type TLSServerHello struct {
	Version           TLSNamedValue   `json:"version"`
	Random            string          `json:"random"`
	SessionID         string          `json:"session_id"`
	CipherSuite       TLSNamedValue   `json:"cipher_suite"`
	CompressionMethod TLSNamedValue   `json:"compression_method"`
	Extensions        json.RawMessage `json:"extensions"`
}

// TLSEncryptedExtensions represents encrypted extensions sent by the server.
type TLSEncryptedExtensions struct {
	Extensions json.RawMessage `json:"extensions"`
}

// TLSCertificateEntry represents a single certificate in a certificate chain.
type TLSCertificateEntry struct {
	CertData   string `json:"cert_data"` // Hex-encoded DER certificate
	Extensions string `json:"extensions"`
}

// TLSCertificate represents a TLS Certificate message.
type TLSCertificate struct {
	CertificateRequestContext string                `json:"certificate_request_context"`
	CertificateList           []TLSCertificateEntry `json:"certificate_list"`
}

// TLSCertificateVerify represents a TLS CertificateVerify message.
type TLSCertificateVerify struct {
	Algorithm TLSNamedValue `json:"algorithm"`
	Signature string        `json:"signature"` // Hex-encoded
}

// TLSFinished represents a TLS Finished message.
type TLSFinished struct {
	VerifyData string `json:"verify_data"` // Hex-encoded
}

// TLSChangeCipherSpec represents a TLS ChangeCipherSpec message.
type TLSChangeCipherSpec struct {
	Data string `json:"data"` // Hex-encoded
}

// TLSHandshakeContent wraps the various handshake message types.
// Only one field will be populated based on Type.
type TLSHandshakeContent struct {
	Type                string
	ClientHello         *TLSClientHello
	ServerHello         *TLSServerHello
	EncryptedExtensions *TLSEncryptedExtensions
	Certificate         *TLSCertificate
	CertificateVerify   *TLSCertificateVerify
	Finished            *TLSFinished
}

// UnmarshalJSON implements custom unmarshaling for TLSHandshakeContent.
func (h *TLSHandshakeContent) UnmarshalJSON(data []byte) error {
	var wrapper struct {
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	h.Type = wrapper.Type

	switch wrapper.Type {
	case TLSHandshakeClientHello:
		h.ClientHello = new(TLSClientHello)
		return json.Unmarshal(wrapper.Content, h.ClientHello)
	case TLSHandshakeServerHello:
		h.ServerHello = new(TLSServerHello)
		return json.Unmarshal(wrapper.Content, h.ServerHello)
	case TLSHandshakeEncryptedExtensions:
		h.EncryptedExtensions = new(TLSEncryptedExtensions)
		return json.Unmarshal(wrapper.Content, h.EncryptedExtensions)
	case TLSHandshakeCertificate:
		h.Certificate = new(TLSCertificate)
		return json.Unmarshal(wrapper.Content, h.Certificate)
	case TLSHandshakeCertificateVerify:
		h.CertificateVerify = new(TLSCertificateVerify)
		return json.Unmarshal(wrapper.Content, h.CertificateVerify)
	case TLSHandshakeFinished:
		h.Finished = new(TLSFinished)
		return json.Unmarshal(wrapper.Content, h.Finished)
	}
	return nil
}

// TLSMessageContent wraps handshake or change_cipher_spec content.
// Check Type to determine which field is populated.
type TLSMessageContent struct {
	Type             string
	Handshake        *TLSHandshakeContent
	ChangeCipherSpec *TLSChangeCipherSpec
}

// UnmarshalJSON implements custom unmarshaling for TLSMessageContent.
func (m *TLSMessageContent) UnmarshalJSON(data []byte) error {
	var wrapper struct {
		Type    string          `json:"type"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	m.Type = wrapper.Type

	switch wrapper.Type {
	case TLSMsgHandshake:
		m.Handshake = new(TLSHandshakeContent)
		return json.Unmarshal(wrapper.Content, m.Handshake)
	case TLSMsgChangeCipherSpec:
		m.ChangeCipherSpec = new(TLSChangeCipherSpec)
		return json.Unmarshal(wrapper.Content, m.ChangeCipherSpec)
	}
	return nil
}

// TLSEvent represents a single TLS event in the connection handshake.
type TLSEvent struct {
	Side string            `json:"side"` // "client" or "server"
	Msg  TLSMessageContent `json:"msg"`
}

// APIError represents an error response from the powhttp API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("powhttp API error %d: %s", e.StatusCode, e.Message)
}

// errorResponse is the JSON structure for API errors.
type errorResponse struct {
	Error string `json:"error"`
}

// DecodeBody decodes a base64-encoded body.
// Returns nil if the input is nil.
func DecodeBody(encoded *string) ([]byte, error) {
	if encoded == nil {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(*encoded)
}
