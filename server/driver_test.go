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

func TestRegisterDriver(t *testing.T) {
	reqBody := DriverRegisterRequest{
		Name:     "testDriver",
		Type:     "testType",
		Callback: "http://localhost:8082/",
		Secret:   "secret123",
	}
	jsonData, _ := json.Marshal(reqBody)

	r := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(jsonData))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	registerDriver(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.StatusCode)
	}
}

func TestDeleteDriver(t *testing.T) {
	knownDrivers["testDriver"] = DriverInfo{Name: "testDriver", Type: "testType", Callback: "http://localhost:8082/", Secret: "secret123"}

	reqBody := DriverDeleteRequest{Name: "testDriver"}
	jsonData, _ := json.Marshal(reqBody)

	r := httptest.NewRequest(http.MethodDelete, "/register", bytes.NewBuffer(jsonData))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	registerDriver(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.StatusCode)
	}

	if _, exists := knownDrivers["testDriver"]; exists {
		t.Errorf("Driver was not deleted")
	}
}

func TestRunDriverInvalidSession(t *testing.T) {
	reqBody := DriverExecutionRequest{
		SessionUUID: "invalid-session",
		DriverType:  "testType",
		Action:      "testAction",
		Parameters:  map[string]any{"key": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)

	r := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	runDriver(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest, got %v", resp.StatusCode)
	}
}

// ----- Test functions -----

// Test that using an invalid HTTP method on /register returns an error.
func TestRegisterDriverInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/register", nil)
	w := httptest.NewRecorder()
	registerDriver(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid method, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test malformed JSON for POST registration.
func TestRegisterDriverMalformedJSON(t *testing.T) {
	malformed := strings.NewReader("{not json}")
	req := httptest.NewRequest(http.MethodPost, "/register", malformed)
	w := httptest.NewRecorder()
	registerDriver(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for malformed JSON, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test malformed JSON for DELETE driver.
func TestDeleteDriverMalformedJSON(t *testing.T) {
	malformed := strings.NewReader("{not json}")
	req := httptest.NewRequest(http.MethodDelete, "/register", malformed)
	w := httptest.NewRecorder()
	registerDriver(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for malformed JSON in delete, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test that a missing callback in POST registration assigns a default callback.
func TestRegisterDriverDefaultCallback(t *testing.T) {
	// Clear any previous driver
	knownDriversMutex.Lock()
	delete(knownDrivers, "defaultCallbackDriver")
	knownDriversMutex.Unlock()

	// Create request with empty callback field.
	reqBody := DriverRegisterRequest{
		Name:   "defaultCallbackDriver",
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
	registerDriver(w, req)

	knownDriversMutex.Lock()
	driver, exists := knownDrivers["defaultCallbackDriver"]
	knownDriversMutex.Unlock()
	if !exists {
		t.Fatalf("Driver not found after registration")
	}

	expectedPrefix := "http://127.0.0.1:8082/"
	if !strings.HasPrefix(driver.Callback, expectedPrefix) {
		t.Errorf("Expected default callback to start with %s, got %s", expectedPrefix, driver.Callback)
	}
}

// Test findDriverByType when multiple drivers exist.
func TestFindDriverByType(t *testing.T) {
	// Reset knownDrivers
	knownDriversMutex.Lock()
	knownDrivers = make(map[string]DriverInfo)
	knownDriversMutex.Unlock()

	driver1 := DriverInfo{Name: "driver1", Type: "alpha", Callback: "http://localhost/alpha/", Secret: "s1"}
	driver2 := DriverInfo{Name: "driver2", Type: "beta", Callback: "http://localhost/beta/", Secret: "s2"}
	knownDriversMutex.Lock()
	knownDrivers["driver1"] = driver1
	knownDrivers["driver2"] = driver2
	knownDriversMutex.Unlock()

	if d := findDriverByType("alpha"); d == nil || d.Name != "driver1" {
		t.Errorf("Expected to find driver1 for type alpha")
	}
	if d := findDriverByType("beta"); d == nil || d.Name != "driver2" {
		t.Errorf("Expected to find driver2 for type beta")
	}
	if d := findDriverByType("gamma"); d != nil {
		t.Errorf("Expected nil for unknown driver type")
	}
}

// Test setupPreconfiguredDriver when configuration is missing.
func TestSetupPreconfiguredDriverMissingConfig(t *testing.T) {
	// Ensure viper does not have the config for this driver.
	viper.Reset()
	driverName := "missingDriver"
	setupPreconfiguredDriver(driverName)
	// Here we expect that no driver is added since callback config is missing.
	knownDriversMutex.Lock()
	_, exists := knownDrivers[driverName]
	knownDriversMutex.Unlock()
	if exists {
		t.Errorf("Driver should not be registered when callback is missing in configuration")
	}
}

// Test setupPreconfiguredDriver with a successful HTTP registration.
// This test uses an HTTP test server to simulate the driver's callback endpoint.
func TestSetupPreconfiguredDriverSuccess(t *testing.T) {
	// Prepare viper configuration for the driver.
	driverName := "testDriver"
	expectedSecret := "secretXYZ"
	viper.Reset()
	viper.Set(fmt.Sprintf("drivers.%s.callback", driverName), "") // We'll override callback below.
	viper.Set(fmt.Sprintf("drivers.%s.secret", driverName), expectedSecret)
	viper.Set("hostname", "127.0.0.1")
	viper.Set("port", 9090)

	var tsURL string
	// Create a test HTTP server to simulate driver's serverConnect endpoint.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the request and respond with valid JSON including the secret.
		var req serverRegistrationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// Use the captured tsURL here.
		respObj := DriverRegisterRequest{
			Name:     driverName,
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
	viper.Set(fmt.Sprintf("drivers.%s.callback", driverName), tsURL)
	// Call setupPreconfiguredDriver.
	setupPreconfiguredDriver(driverName)
	// Verify that the driver was registered.
	knownDriversMutex.Lock()
	driver, exists := knownDrivers[driverName]
	knownDriversMutex.Unlock()
	if !exists {
		t.Fatalf("Expected driver %s to be registered", driverName)
	}
	if driver.Secret != expectedSecret {
		t.Errorf("Expected secret %s, got %s", expectedSecret, driver.Secret)
	}
}

// Test runDriver with missing session id.
func TestRunDriverMissingSession(t *testing.T) {
	reqBody := DriverExecutionRequest{
		SessionUUID: "",
		DriverType:  "testType",
		Action:      "action",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runDriver(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for missing session id, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test runDriver with a malformed session id.
func TestRunDriverMalformedSession(t *testing.T) {
	reqBody := DriverExecutionRequest{
		SessionUUID: "bad-uuid",
		DriverType:  "testType",
		Action:      "action",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runDriver(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for malformed session id, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test runDriver with an unknown session id.
func TestRunDriverUnknownSession(t *testing.T) {
	// Create a valid UUID that is not in our session register.
	validUUID := uuid.New()
	reqBody := DriverExecutionRequest{
		SessionUUID: validUUID.String(),
		DriverType:  "testType",
		Action:      "action",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runDriver(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d for unknown session id, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test runDriver when no supported driver is found.
func TestRunDriverNoSupportedDriver(t *testing.T) {
	// Add a valid session.
	sid := uuid.New()
	sinfo := SessionInfo{
		UUID: sid,
		Context: SessionContext{
			Log: []SessionLogMessage{},
		},
	}
	session_register.addSession(&sinfo)
	reqBody := DriverExecutionRequest{
		SessionUUID: sid.String(),
		DriverType:  "nonexistentType",
		Action:      "action",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runDriver(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("Expected status %d when no supported driver is found, got %d", http.StatusBadGateway, resp.StatusCode)
	}
}

// Test runDriver with a successful driver execution.
// This sets up a dummy driver in knownDrivers and uses an HTTP test server to simulate the driver endpoint.
func TestRunDriverSuccess(t *testing.T) {
	// Setup a dummy session.
	sid := uuid.New()
	sinfo := SessionInfo{
		UUID: sid,
		Context: SessionContext{
			Log: []SessionLogMessage{},
		},
	}
	session_register.addSession(&sinfo)
	// Setup a dummy driver in knownDrivers.
	driverName := "dummyDriver"
	successResponse := DriverExecutionResult{Success: true, Message: "All good"}
	// Create test server to simulate driver's /execute endpoint.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(successResponse)
	}))
	defer ts.Close()

	// Register the dummy driver.
	knownDriversMutex.Lock()
	knownDrivers[driverName] = DriverInfo{
		Name:     driverName,
		Type:     "dummyType",
		Callback: ts.URL + "/", // ensure trailing slash\n",
		Secret:   "dummySecret",
	}
	knownDriversMutex.Unlock()

	reqBody := DriverExecutionRequest{
		SessionUUID: sid.String(),
		DriverType:  "dummyType",
		Action:      "executeSomething",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runDriver(w, req)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	var result DriverExecutionResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if !result.Success {
		t.Errorf("Expected driver execution success")
	}
	// Optionally, check that logs were appended.
	if len(sinfo.Context.Log) == 0 {
		t.Errorf("Expected logs to be recorded in session context")
	}
}

// Test runDriver with a failed driver execution.
func TestRunDriverFailure(t *testing.T) {
	// Setup a dummy session.
	sid := uuid.New()
	sinfo := SessionInfo{
		UUID: sid,
		Context: SessionContext{
			Log: []SessionLogMessage{},
		},
	}
	session_register.addSession(&sinfo)

	// Setup a dummy driver in knownDrivers.
	driverName := "failingDriver"
	failureResponse := DriverExecutionResult{Success: false, Message: "Something went wrong"}
	// Create test server to simulate driver's /execute endpoint.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(failureResponse)
	}))
	defer ts.Close()

	// Register the dummy driver.
	knownDriversMutex.Lock()
	knownDrivers[driverName] = DriverInfo{
		Name:     driverName,
		Type:     "failingType",
		Callback: ts.URL + "/",
		Secret:   "dummySecret",
	}
	knownDriversMutex.Unlock()

	reqBody := DriverExecutionRequest{
		SessionUUID: sid.String(),
		DriverType:  "failingType",
		Action:      "failAction",
		Parameters:  map[string]any{"param": "value"},
	}
	jsonData, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	runDriver(w, req)
	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	var result DriverExecutionResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}
	if result.Success {
		t.Errorf("Expected driver execution failure")
	}
	if result.Message != failureResponse.Message {
		t.Errorf("Expected message %q, got %q", failureResponse.Message, result.Message)
	}
}
