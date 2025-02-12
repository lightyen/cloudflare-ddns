package config

import (
	"encoding/json"
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

func NewFile() error {
	data, err := json.MarshalIndent(DefaultConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath, data, 0644)
}
