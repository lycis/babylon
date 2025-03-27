package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lycis/verify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func init() {
	// Initialize a no-op logger to prevent panics.
	logger = zap.NewNop().Sugar()
}

func TestRegisterActorHandler(t *testing.T) {
	reg := ActorRegister{
		actors: make(map[string]Actor),
	}

	actorData := ActorRegisterRequest{
		Name:     "test-actor",
		Type:     "test-type",
		Callback: "http://localhost:8083/",
		Secret:   "supersecret",
	}

	body, _ := json.Marshal(actorData)
	r := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	registerActor(w, r)

	verify.Number(w.Code).Equal(http.StatusOK)
	verify.Map(reg.actors).Len(1)
	verify.String(reg.actors["test-actor"].Name).Equal("test-actor")
}

func TestDeleteActorHandler(t *testing.T) {
	reg := ActorRegister{
		actors: map[string]Actor{
			"test-actor": {Name: "test-actor", Type: "test-type"},
		},
	}

	deleteReq := ActorDeleteRequest{Name: "test-actor"}
	body, _ := json.Marshal(deleteReq)
	r := httptest.NewRequest(http.MethodDelete, "/register", bytes.NewBuffer(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	registerActor(w, r)

	verify.Number(w.Code).Equal(http.StatusOK)
	verify.Map(reg.actors).Len(0)
}

func TestGetActorByTypeNotFound(t *testing.T) {
	reg := ActorRegister{
		actors: make(map[string]Actor),
	}

	result := reg.GetActorByType("unknown-type")

	verify.Nil(result)
}

func TestSetupPreconfiguredActor(t *testing.T) {
	viper.Set("actors.test-actor.callback", "http://localhost:8083/")
	viper.Set("actors.test-actor.secret", "supersecret")

	setupPreconfiguredActor("test-actor")

	verify.Map(actors.actors).Len(1)
	verify.String(actors.actors["test-actor"].Name).Equal("test-actor")
}
