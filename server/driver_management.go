package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type DriverInfo struct {
	Name     string
	Type     string
	Callback string
}

// list of all known / registered drivers with their callback
var knownDrivers map[string]DriverInfo
var knownDriversMutex sync.Mutex

func init() {
	knownDrivers = make(map[string]DriverInfo)
}

func findDriverByType(t string) *DriverInfo {
	knownDriversMutex.Lock()
	defer knownDriversMutex.Unlock()
	for _, di := range knownDrivers {
		if di.Type == t {
			return &di
		}
	}
	return nil
}

type DriverRegisterRequest struct {
	Name     string `json:"driver"`
	Type     string `json:"type"`
	Callback string `json:"callback"`
}

type DriverDeleteRequest struct {
	Name string `json:"driver"`
}

func registerDriver(w http.ResponseWriter, r *http.Request) {
	knownDriversMutex.Lock()
	defer knownDriversMutex.Unlock()

	if r.Method == http.MethodPost {
		var registerReq DriverRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&registerReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if registerReq.Callback == "" || len(registerReq.Callback) < 1 {
			registerReq.Callback = fmt.Sprintf("http://%s:8082/", strings.Split(r.RemoteAddr, ":")[0])
		}

		knownDrivers[registerReq.Name] = DriverInfo(registerReq)

		logger.With("name", registerReq.Name, "type", registerReq.Type, "callback", registerReq.Callback).Info("New driver registered.")
		return
	} else if r.Method == http.MethodDelete {
		var deleteReq DriverDeleteRequest
		if err := json.NewDecoder(r.Body).Decode(&deleteReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		delete(knownDrivers, deleteReq.Name)
		logger.With("name", deleteReq.Name).Info("Driver delted.")
		return
	}

	http.Error(w, "invalid http method", http.StatusBadRequest)
}
