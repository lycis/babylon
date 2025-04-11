package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"
)

func readConfig(configFile string) {
	viper.SetDefault("port", 8080)
	viper.SetDefault("hostname", "localhost")

	logger.Info("Reading config file.")

	// Extract name and path from the configFile parameter
	absPath, err := filepath.Abs(configFile)
	if err != nil {
		fmt.Println(err)
		logger.With("error", err).Warn("Could not resolve config file path")
		return
	}

	viper.SetConfigFile(absPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println(err)
		logger.With("error", err).Warn("Invalid configuration")
	}
}
