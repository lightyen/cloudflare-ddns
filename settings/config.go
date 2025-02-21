package settings

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const DefaultConfigPath = "config/config.json"

var (
	configExts = []string{".json"}
)

func ConfigPath() string {
	v, exists := os.LookupEnv("CONFIG")
	if exists {
		return v
	}
	return DefaultConfigPath
}

func ReadConfigFile() (config Settings, err error) {
	config, _, err = readConfigFile(ConfigPath())
	return
}

func readConfigFile(filename string) (config Settings, path string, err error) {
	config = Default

	p := filepath.Clean(filename)
	dir, name, ext := filepath.Dir(p), filepath.Base(p), filepath.Ext(p)
	if len(name) > len(ext) {
		name = name[:len(name)-len(ext)]
	}

	for _, ext := range configExts {
		target := filepath.Join(dir, name+ext)
		f, err := os.Open(target)
		if err != nil {
			continue
		}

		buf := make([]byte, 4096)
		n, err := f.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			continue
		}

		switch ext {
		case ".yml", ".yaml":
			return config, "", errors.ErrUnsupported
		case ".json":
			if err := json.Unmarshal(buf[:n], &config); err != nil {
				return config, target, err
			}
			return config, target, nil
		}
	}

	err = os.ErrNotExist
	return
}
