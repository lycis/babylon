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
	if viper.GetBool("security.driver.selfManagement") {
		http.HandleFunc("/driver/", registerDriver)
	} else {
		logger.Info("Driver self-management disabled.")
	}

	// actor functions
	http.HandleFunc("/actor/execute", executeActor)
	if viper.GetBool("security.actor.selfManagement") {
		http.HandleFunc("/actor/", registerActor)
	} else {
		logger.Info("Actor self-management disabled.")
	}

	// reporter functions
	if viper.GetBool("security.reporter.selfManagement") {
		http.HandleFunc("/reporter/", registerReporter)
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

	if viper.IsSet("actors") {
		preconfigActors := viper.GetStringMap("actors")
		for a, _ := range preconfigActors {
			go setupPreconfiguredActor(a)
		}
	}

	if viper.IsSet("reporter") {
		preconfigReporters := viper.GetStringMap("reporter")
		for a, _ := range preconfigReporters {
			go setupPreconfiguredReporter(a)
		}
	}

	port := viper.GetInt("port")
	logger.With("port", port).Info("Server listening")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
