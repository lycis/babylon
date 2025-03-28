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

type Actor struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
}

type ActorRegister struct {
	mutex  sync.Mutex
	actors map[string]Actor
}

var actors ActorRegister

func init() {
	actors = ActorRegister{
		actors: make(map[string]Actor),
	}
}

func (r *ActorRegister) AddActor(a Actor) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.actors[a.Name] = a
	logger.With("name", a.Name, "type", a.Type, "callback", a.Callback).Info("New actor registered.")
}

func (r *ActorRegister) GetActorByType(t string) *Actor {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, a := range r.actors {
		if a.Type == t {
			return &a
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
	Name string `json:"name"`
}

func registerActor(w http.ResponseWriter, r *http.Request) {
	actors.mutex.Lock()
	defer actors.mutex.Unlock()

	if r.Method == http.MethodPost {
		var req ActorRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if !strings.HasSuffix(req.Callback, "/") {
			req.Callback += "/"
		}

		actors.actors[req.Name] = Actor(req)
		logger.With("name", req.Name, "type", req.Type, "callback", req.Callback).Info("New actor registered.")
		return
	} else if r.Method == http.MethodDelete {
		var req ActorDeleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		delete(actors.actors, req.Name)
		logger.With("name", req.Name).Info("Actor deleted.")
		return
	}

	http.Error(w, "invalid http method", http.StatusBadRequest)
}

type ActorExecutionRequest struct {
	ActorType  string         `json:"type"`
	Action     string         `json:"action"`
	Parameters map[string]any `json:"parameters"`
	Session    string         `json:"session"`
}

type ActorExecutionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func executeActor(w http.ResponseWriter, r *http.Request) {
	var req ActorExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	actor := actors.GetActorByType(req.ActorType)
	if actor == nil {
		http.Error(w, "no supported actor", http.StatusBadGateway)
		return
	}

	if len(req.Session) < 1 || req.Session == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
	}

	uuid, err := uuid.Parse(req.Session)
	if err != nil {
		http.Error(w, fmt.Sprintf("malformed session id: %s", err.Error()), http.StatusBadRequest)
		return
	}

	sinfo := session_register.getSession(uuid)
	if sinfo == nil {
		http.Error(w, "unknown session id", http.StatusBadRequest)
	}

	logger.With("session", req.Session, "type", req.ActorType, "action", req.Action).Info("Actor execution request received.")

	actorURL := fmt.Sprintf("%sactor/%s/execute", actor.Callback, actor.Name)
	reqJSON, err := json.Marshal(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sinfo.Context.appendLog(fmt.Sprintf("system::actor::%s", actor.Name), fmt.Sprintf("Executing action '%s'.", req.Action))

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

func setupPreconfiguredActor(name string) {
	actors.mutex.Lock()
	defer actors.mutex.Unlock()

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
	req := ActorRegisterRequest{
		Callback: fmt.Sprintf("http://%s:%d/", viper.GetString("hostname"), viper.GetInt("port")),
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		logger.With("actor", name, "error", err).Error("Failed to marshal server side registration request.")
		return
	}

	resp, err := http.Post(actorURL, "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		logger.With("actor", name, "error", err).Error("Failed to attach server to actor.")
		return
	}

	if resp.StatusCode != http.StatusOK {
		logger.With("actor", name, "statusCode", resp.StatusCode).Error("Failed to attach server to actor. Check actor logs.")
		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.With("actor", name, "error", err).Error("Failed reading actor response on server side registration.")
		return
	}

	var result ActorRegisterRequest
	if err := json.Unmarshal(body, &result); err != nil {
		logger.With("actor", name, "error", err).Error("Failed parsing actor response on server side registration.")
		return
	}

	if result.Secret != secret {
		logger.With("actor", name).Error("Server side actor registration aborted. Invalid secret from actor.")
		return
	}

	actors.actors[name] = Actor(result)
	logger.With("actor", name).Info("Server side actor registered.")
}
