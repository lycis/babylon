package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type sessionRegister struct {
	sessionMutex   sync.Mutex
	activeSessions map[uuid.UUID]*SessionInfo
}

func (r *sessionRegister) addSession(sinfo *SessionInfo) {
	r.sessionMutex.Lock()
	defer r.sessionMutex.Unlock()

	r.activeSessions[sinfo.UUID] = sinfo
}

func (r *sessionRegister) sessionCleanup() {
	ticker := time.Tick(5 * time.Second)
	for now := range ticker {
		logger.Debug("Running session cleanup")
		r.sessionMutex.Lock()
		for _, sinfo := range r.activeSessions {
			if sinfo.lastKeepalive.Before(now.Add(-5 * time.Minute)) {
				logger.With("uuid", sinfo.UUID.String()).Info("Cleaned inactive session.")
				sinfo.Context.appendLog("system::warning", "Session timed out. Missing closing of session or session got stuck?")
				delete(r.activeSessions, sinfo.UUID)
			}
		}
		r.sessionMutex.Unlock()
	}
}

func (r *sessionRegister) getSession(id uuid.UUID) *SessionInfo {
	r.sessionMutex.Lock()
	defer r.sessionMutex.Unlock()
	return r.activeSessions[id]
}

func (r *sessionRegister) removeSession(id uuid.UUID) {
	r.sessionMutex.Lock()
	defer r.sessionMutex.Unlock()
	sendSessionReport(r.activeSessions[id])
	drivers.informEndOfSessioNnid(id)
	informActorsEndOfSession(id)
	delete(r.activeSessions, id)
}

var session_register sessionRegister

type SessionInfo struct {
	UUID          uuid.UUID      `json:"uuid"`
	lastKeepalive time.Time      `json:"-"`
	Context       SessionContext `json:"context"`
}

type SessionContext struct {
	sessionInfo *SessionInfo        `json:"-"`
	mutex       sync.Mutex          `json:"-"`
	Log         []SessionLogMessage `json:"log"`
}

func (c *SessionContext) appendLog(msgtype string, msg string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	msgObj := SessionLogMessage{
		TimeStamp:   time.Now(),
		MessageType: msgtype,
		Message:     msg,
	}
	c.Log = append(c.Log, msgObj)
	go sendLiveLogMessage(c.sessionInfo, msgObj)
}

type SessionLogMessage struct {
	TimeStamp   time.Time `json:"timestamp"`
	MessageType string    `json:"type"`
	Message     string    `json:"message"`
}

func init() {
	session_register.activeSessions = make(map[uuid.UUID]*SessionInfo)
	go session_register.sessionCleanup()
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
	if r.Method != http.MethodGet {
		http.Error(w, "invalid method", http.StatusBadRequest)
		return
	}

	id := uuid.New()
	sinfo := SessionInfo{
		UUID:          id,
		lastKeepalive: time.Now(),
	}

	sinfo.Context = SessionContext{
		sessionInfo: &sinfo,
		Log:         make([]SessionLogMessage, 0),
	}

	session_register.addSession(&sinfo)

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&sinfo)

	logger.With("uuid", sinfo.UUID.String()).Info("New session created.")
}

func handleSessionDetails(w http.ResponseWriter, r *http.Request) {

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

	sinfo := session_register.getSession(uuid)
	if sinfo == nil {
		http.Error(w, "invalid session", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		session_register.removeSession(sinfo.UUID)
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

	logger.With("type", req.Type, "session", sinfo.UUID).Info("Received session context.")
	switch req.Type {
	case "logMessage":
		sinfo.Context.appendLog("message", req.LogMessage)
	default:
		http.Error(w, "invalid context type", http.StatusBadRequest)
	}
}
