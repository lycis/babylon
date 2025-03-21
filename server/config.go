package main

import (
	"fmt"

	"github.com/spf13/viper"
)

func readConfig() {
	viper.SetDefault("port", 8080)
	viper.SetDefault("hostname", "localhost")

	logger.Info("Reading config file.")
	viper.SetConfigName("babylon.yaml")
	viper.AddConfigPath("./")
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println(err)
		logger.With("error", err).Warn("Invalid configuration")
	}
}
