package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Initialize a no-op logger to avoid nil pointer panics.
func init() {
	logger = zap.NewNop().Sugar()
}

// Define a context key type.
type ctxKey string

const sessionIDKey ctxKey = "sessionID"

// newRequestWithSessionID creates an HTTP request with an optional session ID stored in its context.
// If body is nil, it creates an empty buffer.
func newRequestWithSessionID(method, target, sessID string, body *bytes.Buffer) *http.Request {
	if body == nil {
		body = bytes.NewBuffer([]byte{})
	}
	req := httptest.NewRequest(method, target, body)
	if sessID != "" {
		req = req.WithContext(context.WithValue(req.Context(), sessionIDKey, sessID))
	}
	return req
}

// getSessionIDFromRequest retrieves the session id from the request context.
func getSessionIDFromRequest(r *http.Request) string {
	if val := r.Context().Value(sessionIDKey); val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}

// --- Tests for session functions ---

func TestCreateSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/session", nil)
	w := httptest.NewRecorder()
	createSession(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201 Created, got %d", resp.StatusCode)
	}
	var sinfo SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&sinfo); err != nil {
		t.Fatalf("Failed to decode session info: %v", err)
	}
	if sinfo.UUID == uuid.Nil {
		t.Errorf("Expected non-nil UUID")
	}
	// Check that the session is stored.
	stored := session_register.getSession(sinfo.UUID)
	if stored == nil {
		t.Errorf("Session not stored in session_register")
	}
}

func TestHandleSession_WithoutID(t *testing.T) {
	// Calling handleSession with GET should create a session.
	req := httptest.NewRequest(http.MethodGet, "/session", nil)
	w := httptest.NewRecorder()
	handleSession(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201 Created, got %d", resp.StatusCode)
	}
}

func TestHandleSessionDetails_WithEmptyID(t *testing.T) {
	// When no session id is provided, GET should call createSession.
	req := httptest.NewRequest(http.MethodGet, "/session", nil)
	w := httptest.NewRecorder()
	handleSessionDetails(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201 Created when no session id provided, got %d", resp.StatusCode)
	}
}

func TestHandleSessionDetails_GetExisting(t *testing.T) {
	// Create a session manually.
	id := uuid.New()
	sinfo := &SessionInfo{
		UUID:          id,
		lastKeepalive: time.Now(),
		Context:       SessionContext{Log: []SessionLogMessage{}},
	}
	session_register.addSession(sinfo)
	// Simulate a GET request for this session.
	url := fmt.Sprintf("/session/%s", id.String())
	req := newRequestWithSessionID(http.MethodGet, url, id.String(), nil)
	w := httptest.NewRecorder()
	req.SetPathValue("id", id.String())
	handleSessionDetails(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201 Created for GET session details, got %d", resp.StatusCode)
	}
	var retInfo SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&retInfo); err != nil {
		t.Fatalf("Failed to decode session info: %v", err)
	}
	if retInfo.UUID != id {
		t.Errorf("Returned session UUID mismatch, expected %s got %s", id, retInfo.UUID)
	}
}

func TestHandleSessionDetails_Delete(t *testing.T) {
	id := uuid.New()
	sinfo := &SessionInfo{
		UUID:          id,
		lastKeepalive: time.Now(),
		Context:       SessionContext{Log: []SessionLogMessage{}},
	}
	session_register.addSession(sinfo)
	url := fmt.Sprintf("/session/%s", id.String())
	req := newRequestWithSessionID(http.MethodDelete, url, id.String(), nil)
	w := httptest.NewRecorder()
	req.SetPathValue("id", id.String())
	handleSessionDetails(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK for DELETE, got %d", resp.StatusCode)
	}
	if session_register.getSession(id) != nil {
		t.Errorf("Session was not removed")
	}
}

func TestHandleSessionDetails_MalformedID(t *testing.T) {
	badID := "bad-id"
	url := fmt.Sprintf("/session/%s", badID)
	req := newRequestWithSessionID(http.MethodGet, url, badID, nil)
	w := httptest.NewRecorder()
	req.SetPathValue("id", badID)
	handleSessionDetails(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected status 400 for malformed session id, got %d", resp.StatusCode)
	}
}

func TestHandleSessionDetails_UnknownID(t *testing.T) {
	unknownID := uuid.New().String()
	url := fmt.Sprintf("/session/%s", unknownID)
	req := newRequestWithSessionID(http.MethodGet, url, unknownID, nil)
	w := httptest.NewRecorder()
	req.SetPathValue("id", unknownID)
	handleSessionDetails(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected status 404 for unknown session id, got %d", resp.StatusCode)
	}
}

func TestAppendToSession(t *testing.T) {
	// Create a session.
	id := uuid.New()
	sinfo := &SessionInfo{
		UUID:          id,
		lastKeepalive: time.Now(),
		Context:       SessionContext{Log: []SessionLogMessage{}},
	}
	session_register.addSession(sinfo)
	// Create request body for appending a log message.
	reqBody := sessionContextRequest{
		Type:       "logMessage",
		LogMessage: "Test log entry",
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/session", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	appendToSession(w, req, sinfo)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 for appendToSession, got %d", resp.StatusCode)
	}
	if len(sinfo.Context.Log) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(sinfo.Context.Log))
	} else if !strings.Contains(sinfo.Context.Log[0].Message, "Test log entry") {
		t.Errorf("Expected log message to contain 'Test log entry', got %s", sinfo.Context.Log[0].Message)
	}
}
