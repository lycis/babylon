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

type Driver struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
}

type DriverRegister struct {
	mutex   sync.Mutex
	drivers map[string]Driver
}

var drivers DriverRegister

func init() {
	drivers = DriverRegister{
		drivers: make(map[string]Driver),
	}
}

func (r *DriverRegister) AddDriver(a Driver) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.drivers[a.Name] = a
	logger.With("name", a.Name, "type", a.Type, "callback", a.Callback).Info("New driver registered.")
}

func (r *DriverRegister) informEndOfSessioNnid(id uuid.UUID) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for k, driver := range r.drivers {
		logger.With("driver", k, "session", id.String()).Info("Informing driver of session end.")
		driverURL := fmt.Sprintf("%sdriver/%s/session/%s", driver.Callback, driver.Name, id.String())
		req, err := http.NewRequest(http.MethodDelete, driverURL, nil)
		if err != nil {
			logger.With("driver", k, "session", id.String(), "error", err.Error()).Error("Creating request to inform driver of session end failed.")
			continue
		}
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			logger.With("driver", k, "session", id.String(), "error", err.Error()).Error("Informing driver of session end failed.")
			continue
		}
		if resp.StatusCode != http.StatusOK {
			logger.With("driver", k, "session", id.String(), "statusCode", resp.StatusCode).Error("Informing driver of session end failed.")
			continue
		}
	}
}

func (r *DriverRegister) GetDriverByType(t string) *Driver {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, a := range r.drivers {
		if a.Type == t {
			return &a
		}
	}
	return nil
}

type DriverRegisterRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
}

type DriverDeleteRequest struct {
	Name string `json:"name"`
}

func registerDriver(w http.ResponseWriter, r *http.Request) {
	drivers.mutex.Lock()
	defer drivers.mutex.Unlock()

	if r.Method == http.MethodPost {
		var req DriverRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if !strings.HasSuffix(req.Callback, "/") {
			req.Callback += "/"
		}

		drivers.drivers[req.Name] = Driver(req)
		logger.With("name", req.Name, "type", req.Type, "callback", req.Callback).Info("New driver registered.")
		return
	} else if r.Method == http.MethodDelete {
		var req DriverDeleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		delete(drivers.drivers, req.Name)
		logger.With("name", req.Name).Info("Driver deleted.")
		return
	}

	http.Error(w, "invalid http method", http.StatusBadRequest)
}

type DriverExecutionRequest struct {
	DriverType string         `json:"type"`
	Action     string         `json:"action"`
	Parameters map[string]any `json:"parameters"`
	Session    string         `json:"session"`
}

type DriverExecutionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func executeDriver(w http.ResponseWriter, r *http.Request) {
	var req DriverExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	driver := drivers.GetDriverByType(req.DriverType)
	if driver == nil {
		http.Error(w, "no supported driver", http.StatusBadGateway)
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

	logger.With("session", req.Session, "type", req.DriverType, "action", req.Action).Info("Driver execution request received.")

	driverURL := fmt.Sprintf("%sdriver/%s/execute", driver.Callback, driver.Name)
	reqJSON, err := json.Marshal(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sinfo.Context.appendLog(fmt.Sprintf("system::driver::%s", driver.Name), fmt.Sprintf("Executing action '%s'.", req.Action))

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
		sinfo.Context.appendLog(fmt.Sprintf("system::driver::%s", driver.Name), "Driver action: SUCCESS")
	} else {
		sinfo.Context.appendLog(fmt.Sprintf("system::driver::%s", driver.Name), "Driver action: FAILED")
	}

	if len(result.Message) > 0 {
		sinfo.Context.appendLog(fmt.Sprintf("message::driver::%s", driver.Name), result.Message)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func setupPreconfiguredDriver(name string) {
	drivers.mutex.Lock()
	defer drivers.mutex.Unlock()

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
	req := DriverRegisterRequest{
		Callback: fmt.Sprintf("http://%s:%d/", viper.GetString("hostname"), viper.GetInt("port")),
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		logger.With("driver", name, "error", err).Error("Failed to marshal server side registration request.")
		return
	}

	resp, err := http.Post(driverURL, "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		logger.With("driver", name, "error", err).Error("Failed to attach server to driver.")
		return
	}

	if resp.StatusCode != http.StatusOK {
		logger.With("driver", name, "statusCode", resp.StatusCode).Error("Failed to attach server to driver. Check driver logs.")
		return
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.With("driver", name, "error", err).Error("Failed reading driver response on server side registration.")
		return
	}

	var result DriverRegisterRequest
	if err := json.Unmarshal(body, &result); err != nil {
		logger.With("driver", name, "error", err).Error("Failed parsing driver response on server side registration.")
		return
	}

	if result.Secret != secret {
		logger.With("driver", name).Error("Server side driver registration aborted. Invalid secret from driver.")
		return
	}

	drivers.drivers[name] = Driver(result)
	logger.With("driver", name).Info("Server side driver registered.")
}
