package config

import (
	"os"
)

var (
	Version       string
	ConfigPath    = "config/config.json"
	DefaultConfig = Configuration{
		ServerPort:    37211,
		WebRoot:       "www",
		DataDirectory: "data",
	}
)

func init() {
	if v, exists := os.LookupEnv("CONFIG"); exists {
		if v != "" {
			ConfigPath = v
		}
	}
}
