package config

type Configuration struct {
	ServerPort    int    `json:"port" yaml:"port"`
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
