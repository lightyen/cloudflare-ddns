package settings

import (
	"sync/atomic"
)

type Settings struct {
	ServePort      int    `json:"http" yaml:"http" usage:"server port"`
	ServeTLSPort   int    `json:"https" yaml:"https"`
	TLSCertificate string `json:"tls_cert" yaml:"tls_cert"`
	TLSKey         string `json:"tls_key" yaml:"tls_key"`
	TLSPfx         string `json:"tls_pfx" yaml:"tls_pfx"`

	WebRoot       string `json:"www" yaml:"www"`
	DataDirectory string `json:"data" yaml:"data"`

	Email      string   `json:"email" yaml:"email"`
	Token      string   `json:"token" yaml:"token"`
	ZoneID     string   `json:"zone" yaml:"zone"`
	Records    []Record `json:"records" yaml:"records" cli:",ignored"`
	StaticIPv6 string   `json:"static_ipv6" yaml:"static_ipv6"`
}

type Record struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Proxied bool   `json:"proxied"`
}

var (
	Version   string
	BuildTime string
	Default   = Settings{
		ServePort:     80,
		ServeTLSPort:  443,
		WebRoot:       "www",
		DataDirectory: "data",
	}
)

var (
	value atomic.Value
)

func Load() error {
	m, _, err := readConfigFile(ConfigPath())
	value.Store(m)
	return err
}

func Value() Settings {
	return value.Load().(Settings)
}
