package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func init() {
	// Create a no-op logger so that logger calls don't panic.
	logger = zap.NewNop().Sugar()
}

func TestRegisterActor(t *testing.T) {
	reqBody := ActorRegisterRequest{
		Name:     "testActor",
		Type:     "testType",
		Callback: "http://localhost:8082/",
		Secret:   "secret123",
	}
	jsonData, _ := json.Marshal(reqBody)

	r := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(jsonData))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	registerActor(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.StatusCode)
	}
}

func TestDeleteActor(t *testing.T) {
	knownActors["testActor"] = ActorInfo{Name: "testActor", Type: "testType", Callback: "http://localhost:8082/", Secret: "secret123"}

	reqBody := ActorDeleteRequest{Name: "testActor"}
	jsonData, _ := json.Marshal(reqBody)

	r := httptest.NewRequest(http.MethodDelete, "/register", bytes.NewBuffer(jsonData))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	registerActor(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.StatusCode)
	}

	if _, exists := knownActors["testActor"]; exists {
		t.Errorf("Actor was not deleted")
	}
}

func TestRunActorInvalidSession(t *testing.T) {
	reqBody := ActorExecutionRequest{
		SessionUUID: "invalid-session",
		ActorType:   "testType",
		Action:      "testAction",
		Parameters:  map[string]any{"key": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)

	r := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	runActor(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest, got %v", resp.StatusCode)
	}
}

// ----- Test functions -----

// Test that using an invalid HTTP method on /register returns an error.
func TestRegisterActorInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	w := httptest.NewRecorder()
	registerActor(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid method, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test malformed JSON for POST registration.
func TestRegisterActorMalformedJSON(t *testing.T) {
	malformed := strings.NewReader("{not json}")
	req := httptest.NewRequest(http.MethodPost, "/register", malformed)
	w := httptest.NewRecorder()
	registerActor(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for malformed JSON, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test malformed JSON for DELETE actor.
func TestDeleteActorMalformedJSON(t *testing.T) {
	malformed := strings.NewReader("{not json}")
	req := httptest.NewRequest(http.MethodDelete, "/register", malformed)
	w := httptest.NewRecorder()
	registerActor(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for malformed JSON in delete, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test that a missing callback in POST registration assigns a default callback.
func TestRegisterActorDefaultCallback(t *testing.T) {
	// Clear any previous actor
	knownActorsMutex.Lock()
	delete(knownActors, "defaultCallbackActor")
	knownActorsMutex.Unlock()

	// Create request with empty callback field.
	reqBody := ActorRegisterRequest{
		Name:   "defaultCallbackActor",
		Type:   "testType",
		Secret: "secret123",
		// Callback omitted/empty
		Callback: "",
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(jsonData))
	// Simulate a remote address (only IP part is used for default callback)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	registerActor(w, req)

	knownActorsMutex.Lock()
	actor, exists := knownActors["defaultCallbackActor"]
	knownActorsMutex.Unlock()
	if !exists {
		t.Fatalf("Actor not found after registration")
	}

	expectedPrefix := "http://127.0.0.1:8082/"
	if !strings.HasPrefix(actor.Callback, expectedPrefix) {
		t.Errorf("Expected default callback to start with %s, got %s", expectedPrefix, actor.Callback)
	}
}

// Test findActorByType when multiple actors exist.
func TestFindActorByType(t *testing.T) {
	// Reset knownActors
	knownActorsMutex.Lock()
	knownActors = make(map[string]ActorInfo)
	knownActorsMutex.Unlock()

	actor1 := ActorInfo{Name: "actor1", Type: "alpha", Callback: "http://localhost/alpha/", Secret: "s1"}
	actor2 := ActorInfo{Name: "actor2", Type: "beta", Callback: "http://localhost/beta/", Secret: "s2"}
	knownActorsMutex.Lock()
	knownActors["actor1"] = actor1
	knownActors["actor2"] = actor2
	knownActorsMutex.Unlock()

	if d := findActorByType("alpha"); d == nil || d.Name != "actor1" {
		t.Errorf("Expected to find actor1 for type alpha")
	}
	if d := findActorByType("beta"); d == nil || d.Name != "actor2" {
		t.Errorf("Expected to find actor2 for type beta")
	}
	if d := findActorByType("gamma"); d != nil {
		t.Errorf("Expected nil for unknown actor type")
	}
}

// Test setupPreconfiguredActor when configuration is missing.
func TestSetupPreconfiguredActorMissingConfig(t *testing.T) {
	// Ensure viper does not have the config for this actor.
	viper.Reset()
	actorName := "missingActor"
	setupPreconfiguredActor(actorName)
	// Here we expect that no actor is added since callback config is missing.
	knownActorsMutex.Lock()
	_, exists := knownActors[actorName]
	knownActorsMutex.Unlock()
	if exists {
		t.Errorf("Actor should not be registered when callback is missing in configuration")
	}
}

// Test setupPreconfiguredActor with a successful HTTP registration.
// This test uses an HTTP test server to simulate the actor's callback endpoint.
func TestSetupPreconfiguredActorSuccess(t *testing.T) {
	// Prepare viper configuration for the actor.
	actorName := "testActor"
	expectedSecret := "secretXYZ"
	viper.Reset()
	viper.Set(fmt.Sprintf("actors.%s.callback", actorName), "") // We'll override callback below.
	viper.Set(fmt.Sprintf("actors.%s.secret", actorName), expectedSecret)
	viper.Set("hostname", "127.0.0.1")
	viper.Set("port", 9090)

	var tsURL string
	// Create a test HTTP server to simulate actor's serverConnect endpoint.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the request and respond with valid JSON including the secret.
		var req serverRegistrationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// Use the captured tsURL here.
		respObj := ActorRegisterRequest{
			Name:     actorName,
			Type:     "someType",
			Callback: tsURL + "/", // echo back test server URL with trailing slash
			Secret:   expectedSecret,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respObj)
	}))
	defer ts.Close()

	// Capture the test server URL.
	tsURL = ts.URL

	// Set the callback in viper to point to our test server.
	viper.Set(fmt.Sprintf("actors.%s.callback", actorName), tsURL)
	// Call setupPreconfiguredActor.
	setupPreconfiguredActor(actorName)
	// Verify that the actor was registered.
	knownActorsMutex.Lock()
	actor, exists := knownActors[actorName]
	knownActorsMutex.Unlock()
	if !exists {
		t.Fatalf("Expected actor %s to be registered", actorName)
	}
	if actor.Secret != expectedSecret {
		t.Errorf("Expected secret %s, got %s", expectedSecret, actor.Secret)
	}
}

// Test runActor with missing session id.
func TestRunActorMissingSession(t *testing.T) {
	reqBody := ActorExecutionRequest{
		SessionUUID: "",
		ActorType:   "testType",
		Action:      "action",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runActor(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for missing session id, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test runActor with a malformed session id.
func TestRunActorMalformedSession(t *testing.T) {
	reqBody := ActorExecutionRequest{
		SessionUUID: "bad-uuid",
		ActorType:   "testType",
		Action:      "action",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runActor(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for malformed session id, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test runActor with an unknown session id.
func TestRunActorUnknownSession(t *testing.T) {
	// Create a valid UUID that is not in our session register.
	validUUID := uuid.New()
	reqBody := ActorExecutionRequest{
		SessionUUID: validUUID.String(),
		ActorType:   "testType",
		Action:      "action",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runActor(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for unknown session id, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test runActor when no supported actor is found.
func TestRunActorNoSupportedActor(t *testing.T) {
	// Add a valid session.
	sid := uuid.New()
	sinfo := SessionInfo{
		UUID: sid,
		Context: SessionContext{
			Log: []SessionLogMessage{},
		},
	}
	session_register.addSession(&sinfo)
	reqBody := ActorExecutionRequest{
		SessionUUID: sid.String(),
		ActorType:   "nonexistentType",
		Action:      "action",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runActor(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("Expected status %d when no supported actor is found, got %d", http.StatusBadGateway, resp.StatusCode)
	}
}

// Test runActor with a successful actor execution.
// This sets up a dummy actor in knownActors and uses an HTTP test server to simulate the actor endpoint.
func TestRunActorSuccess(t *testing.T) {
	// Setup a dummy session.
	sid := uuid.New()
	sinfo := SessionInfo{
		UUID: sid,
		Context: SessionContext{
			Log: []SessionLogMessage{},
		},
	}
	session_register.addSession(&sinfo)
	// Setup a dummy actor in knownActors.
	actorName := "dummyActor"
	successResponse := ActorExecutionResult{Success: true, Message: "All good"}
	// Create test server to simulate actor's /execute endpoint.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(successResponse)
	}))
	defer ts.Close()

	// Register the dummy actor.
	knownActorsMutex.Lock()
	knownActors[actorName] = ActorInfo{
		Name:     actorName,
		Type:     "dummyType",
		Callback: ts.URL + "/", // ensure trailing slash\n",
		Secret:   "dummySecret",
	}
	knownActorsMutex.Unlock()

	reqBody := ActorExecutionRequest{
		SessionUUID: sid.String(),
		ActorType:   "dummyType",
		Action:      "executeSomething",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runActor(w, req)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	var result ActorExecutionResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if !result.Success {
		t.Errorf("Expected actor execution success")
	}
	// Optionally, check that logs were appended.
	if len(sinfo.Context.Log) == 0 {
		t.Errorf("Expected logs to be recorded in session context")
	}
}

// Test runActor with a failed actor execution.
func TestRunActorFailure(t *testing.T) {
	// Setup a dummy session.
	sid := uuid.New()
	sinfo := SessionInfo{
		UUID: sid,
		Context: SessionContext{
			Log: []SessionLogMessage{},
		},
	}
	session_register.addSession(&sinfo)

	// Setup a dummy actor in knownActors.
	actorName := "failingActor"
	failureResponse := ActorExecutionResult{Success: false, Message: "Something went wrong"}
	// Create test server to simulate actor's /execute endpoint.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(failureResponse)
	}))
	defer ts.Close()

	// Register the dummy actor.
	knownActorsMutex.Lock()
	knownActors[actorName] = ActorInfo{
		Name:     actorName,
		Type:     "failingType",
		Callback: ts.URL + "/",
		Secret:   "dummySecret",
	}
	knownActorsMutex.Unlock()

	reqBody := ActorExecutionRequest{
		SessionUUID: sid.String(),
		ActorType:   "failingType",
		Action:      "failAction",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runActor(w, req)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	var result ActorExecutionResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if result.Success {
		t.Errorf("Expected actor execution failure")
	}
	if result.Message != failureResponse.Message {
		t.Errorf("Expected message %q, got %q", failureResponse.Message, result.Message)
	}
}
