package config

import (
	"os"

	"go.uber.org/zap/zapcore"
)

var (
	Config   Configuration
	LogLevel zapcore.Level
	Version  string
)

type Record struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Proxied bool   `json:"proxied"`
}

type Configuration struct {
	ServerPort    string `json:"port" yaml:"port" default:"37211"`
	WebRoot       string `json:"www" yaml:"www" default:"www"`
	DataDirectory string `json:"data" yaml:"data" default:"data"`

	Email      string   `json:"email" yaml:"email" usage:"User Email"`
	Token      string   `json:"token" yaml:"token" usage:"API Token"`
	ZoneID     string   `json:"zone" yaml:"zone" usage:"Zone ID"`
	Records    []Record `json:"records" yaml:"records"`
	StaticIPv6 string   `json:"static_ipv6" yaml:"static_ipv6"`
}

func init() {
	if v, exists := os.LookupEnv("LOG_LEVEL"); exists {
		_ = LogLevel.Set(v)
	}
	if v, exists := os.LookupEnv("CONFIG"); exists {
		if v != "" {
			configPath = v
		}
	}
}
