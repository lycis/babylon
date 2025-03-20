package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

// DriverExecutionRequest sent by the test script.
type DriverExecutionRequest struct {
	SessionUUID string         `json:"session"`
	DriverType  string         `json:"type"`
	Action      string         `json:"action"`
	Parameters  map[string]any `json:"parameters"`
}

type DriverExecutionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func runDriver(w http.ResponseWriter, r *http.Request) {
	var testReq DriverExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&testReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(testReq.SessionUUID) < 1 || testReq.SessionUUID == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
	}

	logger.With("session", testReq.SessionUUID, "type", testReq.DriverType, "action", testReq.Action).Info("Driver execution request received.")

	uuid, err := uuid.Parse(testReq.SessionUUID)
	if err != nil {
		http.Error(w, fmt.Sprintf("malformed session id: %s", err.Error()), http.StatusBadRequest)
		return
	}

	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	sinfo, exists := activeSessions[uuid]
	if !exists {
		http.Error(w, "unknown session id", http.StatusBadRequest)
	}

	driver := findDriverByType(testReq.DriverType)
	if driver == nil {
		http.Error(w, "no supported driver", http.StatusBadGateway)
		return
	}

	appendLogMessageToSession(sinfo, fmt.Sprintf("Executing action '%s' on driver '%s'.", testReq.Action, driver.Name))

	// Forward the request to the BookingServiceDriver service.
	driverURL := fmt.Sprintf("%sdriver/%s/execute", driver.Callback, driver.Name)
	reqJSON, err := json.Marshal(testReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := http.Post(driverURL, "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var result DriverExecutionResult
	if err := json.Unmarshal(body, &result); err != nil {
		http.Error(w, err.Error(), http.StatusFailedDependency)
		return
	}

	if result.Success {
		appendLogMessageToSession(sinfo, "Driver action: SUCCESS")
	} else {
		appendLogMessageToSession(sinfo, "Driver action: FAILED")
	}

	if len(result.Message) > 0 {
		appendLogMessageToSession(sinfo, result.Message)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}
