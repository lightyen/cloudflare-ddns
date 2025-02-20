package config

import (
	"encoding/json"
	"os"
)

type Configuration struct {
	ServePort      int    `json:"http" yaml:"http" usage:"server port"`
	ServeTLSPort   int    `json:"https" yaml:"https"`
	TLSCertificate string `json:"tls-cert" yaml:"tls-cert"`
	TLSKey         string `json:"tls-key" yaml:"tls-key"`
	TLSPfx         string `json:"tls-pfx" yaml:"tls-pfx"`

	WebRoot       string `json:"www" yaml:"www"`
	DataDirectory string `json:"data" yaml:"data"`

	Email      string   `json:"email" yaml:"email"`
	Token      string   `json:"token" yaml:"token"`
	ZoneID     string   `json:"zone" yaml:"zone"`
	Records    []Record `json:"records" yaml:"records"`
	StaticIPv6 string   `json:"static_ipv6" yaml:"static_ipv6"`
}

type Record struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Proxied bool   `json:"proxied"`
}

var (
	Version       string
	PrintVersion  bool
	BuildTime     string
	ConfigPath    = "config/config.json"
	DefaultConfig = Configuration{
		ServePort:     80,
		ServeTLSPort:  443,
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
