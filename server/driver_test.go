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

func TestRegisterDriverHandler(t *testing.T) {
	reg := DriverRegister{
		drivers: make(map[string]Driver),
	}

	driverData := DriverRegisterRequest{
		Name:     "test-driver",
		Type:     "test-type",
		Callback: "http://localhost:8083/",
		Secret:   "supersecret",
	}

	body, _ := json.Marshal(driverData)
	r := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	registerDriver(w, r)

	verify.Number(w.Code).Equal(http.StatusOK)
	verify.Map(reg.drivers).Len(1)
	verify.String(reg.drivers["test-driver"].Name).Equal("test-driver")
}

func TestDeleteDriverHandler(t *testing.T) {
	reg := DriverRegister{
		drivers: map[string]Driver{
			"test-driver": {Name: "test-driver", Type: "test-type"},
		},
	}

	deleteReq := DriverDeleteRequest{Name: "test-driver"}
	body, _ := json.Marshal(deleteReq)
	r := httptest.NewRequest(http.MethodDelete, "/register", bytes.NewBuffer(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	registerDriver(w, r)

	verify.Number(w.Code).Equal(http.StatusOK)
	verify.Map(reg.drivers).Len(0)
}

func TestGetDriverByTypeNotFound(t *testing.T) {
	reg := DriverRegister{
		drivers: make(map[string]Driver),
	}

	result := reg.GetDriverByType("unknown-type")

	verify.Nil(result)
}

func TestSetupPreconfiguredDriver(t *testing.T) {
	viper.Set("drivers.test-driver.callback", "http://localhost:8083/")
	viper.Set("drivers.test-driver.secret", "supersecret")

	setupPreconfiguredDriver("test-driver")

	verify.Map(drivers.drivers).Len(1)
	verify.String(drivers.drivers["test-driver"].Name).Equal("test-driver")
}
