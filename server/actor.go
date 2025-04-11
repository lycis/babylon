package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

type ActorInfo struct {
	Name     string
	Type     string
	Callback string
	Secret   string
}

// list of all known / registered actors with their callback
var knownActors map[string]ActorInfo
var knownActorsMutex sync.Mutex

func informActorsEndOfSession(id uuid.UUID) {
	knownActorsMutex.Lock()
	defer knownActorsMutex.Unlock()

	for k, driver := range knownActors {
		logger.With("actor", k, "session", id.String()).Info("Informing actor of session end.")
		driverURL := fmt.Sprintf("%sactor/%s/session/%s", driver.Callback, driver.Name, id.String())
		req, err := http.NewRequest(http.MethodDelete, driverURL, nil)
		if err != nil {
			logger.With("actor", k, "session", id.String(), "error", err.Error()).Error("Creating request to inform actor of session end failed.")
			continue
		}
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			logger.With("cator", k, "session", id.String(), "error", err.Error()).Error("Informing actor of session end failed.")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			logger.With("cator", k, "session", id.String(), "statusCode", resp.StatusCode).Error("Informing actor of session end failed.")
			continue
		}
	}
}

func init() {
	knownActors = make(map[string]ActorInfo)
}

func findActorByType(t string) *ActorInfo {
	knownActorsMutex.Lock()
	defer knownActorsMutex.Unlock()
	for _, di := range knownActors {
		if di.Type == t {
			return &di
		}
	}
	return nil
}

type ActorRegisterRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
}

type ActorDeleteRequest struct {
	Name string `json:"actor"`
}

func registerActor(w http.ResponseWriter, r *http.Request) {
	knownActorsMutex.Lock()
	defer knownActorsMutex.Unlock()

	if r.Method == http.MethodPost {
		var registerReq ActorRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&registerReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if registerReq.Callback == "" || len(registerReq.Callback) < 1 {
			registerReq.Callback = fmt.Sprintf("http://%s:8082/", strings.Split(r.RemoteAddr, ":")[0])
		}

		knownActors[registerReq.Name] = ActorInfo(registerReq)

		logger.With("name", registerReq.Name, "type", registerReq.Type, "callback", registerReq.Callback).Info("New actor registered.")
		return
	} else if r.Method == http.MethodDelete {
		var deleteReq ActorDeleteRequest
		if err := json.NewDecoder(r.Body).Decode(&deleteReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		delete(knownActors, deleteReq.Name)
		logger.With("name", deleteReq.Name).Info("Actor delted.")
		return
	}

	http.Error(w, "invalid http method", http.StatusBadRequest)
}

type serverRegistrationRequest struct {
	Callback string `json:"callback"`
}

func setupPreconfiguredActor(name string) {
	knownActorsMutex.Lock()
	defer knownActorsMutex.Unlock()

	if !viper.IsSet(fmt.Sprintf("actors.%s.callback", name)) {
		logger.With("actor", name).Error("Preconfigured actor is missing callback.")
		return
	}

	callback := viper.GetString(fmt.Sprintf("actors.%s.callback", name))
	secret := viper.GetString(fmt.Sprintf("actors.%s.secret", name))
	if !viper.IsSet(fmt.Sprintf("actors.%s.secret", name)) {
		secret = ""
	}

	if !strings.HasSuffix(callback, "/") {
		callback += "/"
	}

	actorURL := fmt.Sprintf("%sactor/%s/serverConnect", callback, name)
	req := serverRegistrationRequest{
		Callback: fmt.Sprintf("http://%s:%d/", viper.GetString("hostname"), viper.GetInt("port")),
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		logger.With("actorName", name, "error", err).Error("Failed to marshal server side registration request.")
		return
	}

	resp, err := http.Post(actorURL, "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		logger.With("actorName", name, "error", err).Error("Failed to attach server to actor.")
		return
	}

	if resp.StatusCode != http.StatusOK {
		logger.With("actorName", name, "statusCode", resp.StatusCode).Error("Failed to attach server to actor. Check actor logs.")
		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.With("actorName", name, "error", err).Error("Failed reading actor response on server side registration.")
		return
	}

	var result ActorRegisterRequest
	if err := json.Unmarshal(body, &result); err != nil {
		logger.With("actorName", name, "error", err).Error("Failed parsing actor response on server side registration.")
		return
	}

	if result.Secret != secret {
		logger.With("actorName", name).Error("Server side actor registration aborted. Invalid secret from actor.")
		return
	}

	knownActors[name] = ActorInfo(result)
	logger.With("actorName", name).Info("Server side actor registered.")
}

// ActorExecutionRequest sent by the test script.
type ActorExecutionRequest struct {
	SessionUUID string         `json:"session"`
	ActorType   string         `json:"type"`
	Action      string         `json:"action"`
	Parameters  map[string]any `json:"parameters"`
}

type ActorExecutionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func runActor(w http.ResponseWriter, r *http.Request) {
	var testReq ActorExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&testReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(testReq.SessionUUID) < 1 || testReq.SessionUUID == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
	}

	logger.With("session", testReq.SessionUUID, "type", testReq.ActorType, "action", testReq.Action).Info("Actor execution request received.")

	uuid, err := uuid.Parse(testReq.SessionUUID)
	if err != nil {
		http.Error(w, fmt.Sprintf("malformed session id: %s", err.Error()), http.StatusBadRequest)
		return
	}

	sinfo := session_register.getSession(uuid)
	if sinfo == nil {
		http.Error(w, "unknown session id", http.StatusBadRequest)
	}

	actor := findActorByType(testReq.ActorType)
	if actor == nil {
		http.Error(w, "no supported actor", http.StatusBadGateway)
		return
	}

	sinfo.Context.appendLog(fmt.Sprintf("system::actor::%s", actor.Name), fmt.Sprintf("Executing action '%s'.", testReq.Action))

	// Forward the request to the BookingServiceActor service.
	actorURL := fmt.Sprintf("%sactor/%s/execute", actor.Callback, actor.Name)
	reqJSON, err := json.Marshal(testReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := http.Post(actorURL, "application/json", bytes.NewBuffer(reqJSON))
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

	var result ActorExecutionResult
	if err := json.Unmarshal(body, &result); err != nil {
		http.Error(w, err.Error(), http.StatusFailedDependency)
		return
	}

	if result.Success {
		sinfo.Context.appendLog(fmt.Sprintf("system::actor::%s", actor.Name), "Actor action: SUCCESS")
	} else {
		sinfo.Context.appendLog(fmt.Sprintf("system::actor::%s", actor.Name), "Actor action: FAILED")
	}

	if len(result.Message) > 0 {
		sinfo.Context.appendLog(fmt.Sprintf("message::actor::%s", actor.Name), result.Message)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}
