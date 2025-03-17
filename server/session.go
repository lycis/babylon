package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

var sessionMutex sync.Mutex
var activeSessions map[uuid.UUID]*SessionInfo

type SessionInfo struct {
	UUID          uuid.UUID      `json:"uuid"`
	lastKeepalive time.Time      `json:"-"`
	Context       SessionContext `json:"context"`
}

type SessionContext struct {
	Log []SessionLogMessage
}

type SessionLogMessage struct {
	TimeStamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

func init() {
	activeSessions = make(map[uuid.UUID]*SessionInfo)
	go sessionCleanup()
}

func sessionCleanup() {
	ticker := time.Tick(5 * time.Second)
	for now := range ticker {
		logger.Debug("Running session cleanup")
		sessionMutex.Lock()
		for _, sinfo := range activeSessions {
			if sinfo.lastKeepalive.Before(now.Add(-5 * time.Minute)) {
				logger.With("uuid", sinfo.UUID.String()).Info("Cleaned inactive session.")
				delete(activeSessions, sinfo.UUID)
			}
		}
		sessionMutex.Unlock()
	}
}

func handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		createSession(w, r)
	} else {
		http.Error(w, "invalid method", http.StatusBadRequest)
		return
	}
}

func createSession(w http.ResponseWriter, r *http.Request) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	id := uuid.New()
	sinfo := SessionInfo{
		UUID:          id,
		lastKeepalive: time.Now(),
		Context: SessionContext{
			Log: make([]SessionLogMessage, 0),
		},
	}

	activeSessions[id] = &sinfo

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sinfo)

	logger.With("uuid", sinfo.UUID.String()).Info("New session created.")
}

func handleSessionDetails(w http.ResponseWriter, r *http.Request) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	sid := r.PathValue("id")
	if len(sid) == 0 {
		if r.Method == http.MethodGet {
			createSession(w, r)
			return
		}
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	uuid, err := uuid.Parse(sid)
	if err != nil {
		http.Error(w, fmt.Sprintf("malformed session id: %s", err), http.StatusBadRequest)
		return
	}

	sinfo, exists := activeSessions[uuid]
	if !exists {
		http.Error(w, "invalid session", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		delete(activeSessions, sinfo.UUID)
		logger.With("uuid", sinfo.UUID.String()).Info("Session deleted.")
	case http.MethodPost:
		appendToSession(w, r, sinfo)
	case http.MethodGet:
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sinfo)
	default:
		http.Error(w, "invalid method", http.StatusBadRequest)
	}
}

type sessionContextRequest struct {
	Type       string `json:"type"`
	LogMessage string `json:"logMessage"`
}

func appendToSession(w http.ResponseWriter, r *http.Request, sinfo *SessionInfo) {
	var req sessionContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logger.With("type", req.Type).Info("Received session context.")
	switch req.Type {
	case "logMessage":
		appendLogMessageToSession(sinfo, req.LogMessage)
	default:
		http.Error(w, "invalid context type", http.StatusBadRequest)
	}
}

func appendLogMessageToSession(sinfo *SessionInfo, msg string) {
	sinfo.Context.Log = append(sinfo.Context.Log, SessionLogMessage{
		TimeStamp: time.Now(),
		Message:   msg,
	})
}
