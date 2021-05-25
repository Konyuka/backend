package config

import (
	"log"

	"github.com/spf13/viper"
)

var (
	config *viper.Viper
	err    error
)

func init() {

	// instantiate config
	config = viper.New()

	config.SetConfigType("toml")
	config.SetConfigName("config")

	// Look in . current working dir
	config.AddConfigPath(".")
	config.AddConfigPath("..")

	// Load config file
	if err = config.ReadInConfig(); err != nil {
		log.Printf("Error reading config file: %s \n", err)
	}

}

// GetConfig -
func GetConfig() *viper.Viper {
	return config
}
