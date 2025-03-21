package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type DriverInfo struct {
	Name     string
	Type     string
	Callback string
	Secret   string
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
	Secret   string `json:"secret"`
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

type serverRegistrationRequest struct {
	Callback string `json:"callback"`
}

func setupPreconfiguredDriver(name string) {
	knownDriversMutex.Lock()
	defer knownDriversMutex.Unlock()

	if !viper.IsSet(fmt.Sprintf("drivers.%s.callback", name)) {
		logger.With("driver", name).Error("Preconfigured driver is missing callback.")
		return
	}

	callback := viper.GetString(fmt.Sprintf("drivers.%s.callback", name))
	secret := viper.GetString(fmt.Sprintf("drivers.%s.secret", name))
	if !viper.IsSet(fmt.Sprintf("drivers.%s.secret", name)) {
		secret = ""
	}

	if !strings.HasSuffix(callback, "/") {
		callback += "/"
	}

	driverURL := fmt.Sprintf("%sdriver/%s/serverConnect", callback, name)
	req := serverRegistrationRequest{
		Callback: fmt.Sprintf("http://%s:%d/", viper.GetString("hostname"), viper.GetInt("port")),
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		logger.With("driverName", name, "error", err).Error("Failed to marshal server side registration request.")
		return
	}

	resp, err := http.Post(driverURL, "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		logger.With("driverName", name, "error", err).Error("Failed to attach server to driver.")
		return
	}

	if resp.StatusCode != http.StatusOK {
		logger.With("driverName", name, "statusCode", resp.StatusCode).Error("Failed to attach server to driver. Check driver logs.")
		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.With("driverName", name, "error", err).Error("Failed reading driver response on server side registration.")
		return
	}

	var result DriverRegisterRequest
	if err := json.Unmarshal(body, &result); err != nil {
		logger.With("driverName", name, "error", err).Error("Failed parsing driver response on server side registration.")
		return
	}

	if result.Secret != secret {
		logger.With("driverName", name).Error("Server side driver registration aborted. Invalid secret from driver.")
		return
	}

	knownDrivers[name] = DriverInfo(result)
	logger.With("driverName", name).Info("Server side driver registered.")
	return
}
