package main

import (
	"log"
	"net/http"

	"go.uber.org/zap"
)

var logger *zap.SugaredLogger

func main() {
	l, _ := zap.NewDevelopment()
	defer l.Sync()

	logger = l.Sugar().WithOptions(zap.IncreaseLevel(zap.DebugLevel))

	// Driver functions
	http.HandleFunc("/driver/execute", runDriver)
	http.HandleFunc("/driver/", registerDriver)

	// session management
	http.HandleFunc("/session", handleSession)
	http.HandleFunc("/session/{id}", handleSessionDetails)

	logger.Info("Server listening un :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
