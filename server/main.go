package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var logger *zap.SugaredLogger

func main() {
	l, _ := zap.NewDevelopment()
	defer l.Sync()

	logger = l.Sugar().WithOptions(zap.IncreaseLevel(zap.DebugLevel))

	// update configuration
	readConfig()

	// Driver functions
	http.HandleFunc("/driver/execute", runDriver)
	if viper.GetBool("driver.driverSelfManagement") {
		http.HandleFunc("/driver/", registerDriver)
	} else {
		logger.Info("Driver self-management disabled.")
	}

	// session management
	http.HandleFunc("/session", handleSession)
	http.HandleFunc("/session/{id}", handleSessionDetails)

	if viper.IsSet("drivers") {
		preconfigDrivers := viper.GetStringMap("drivers")
		for driver, _ := range preconfigDrivers {
			go setupPreconfiguredDriver(driver)
		}
	}

	port := viper.GetInt("port")
	logger.With("port", port).Info("Server listening")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
