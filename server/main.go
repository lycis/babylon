package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var logger *zap.SugaredLogger

func main() {
	l, _ := zap.NewDevelopment()
	defer l.Sync()

	logger = l.Sugar().WithOptions(zap.IncreaseLevel(zap.DebugLevel))

	// update configuration
	configFile := "babylon.yaml" // default

	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	readConfig(configFile)

	// Actor functions
	http.HandleFunc("/actor/execute", runActor)
	if viper.GetBool("security.actor.selfManagement") {
		http.HandleFunc("/actor/", registerActor)
	} else {
		logger.Info("Actor self-management disabled.")
	}

	// driver functions
	http.HandleFunc("/driver/execute", executeDriver)
	if viper.GetBool("security.driver.selfManagement") {
		http.HandleFunc("/driver/", registerDriver)
	} else {
		logger.Info("Driver self-management disabled.")
	}

	// reporter functions
	if viper.GetBool("security.reporter.selfManagement") {
		http.HandleFunc("/reporter/", registerReporter)
	}

	// session management
	http.HandleFunc("/session", handleSession)
	http.HandleFunc("/session/{id}", handleSessionDetails)

	if viper.IsSet("actors") {
		preconfigActors := viper.GetStringMap("actors")
		for actor, _ := range preconfigActors {
			go setupPreconfiguredActor(actor)
		}
	}

	if viper.IsSet("drivers") {
		preconfigDrivers := viper.GetStringMap("drivers")
		for a, _ := range preconfigDrivers {
			go setupPreconfiguredDriver(a)
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
