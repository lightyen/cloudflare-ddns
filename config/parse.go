package config

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
)

var (
	configuration   atomic.Value
	ErrRecordFormat = errors.New("wrong record format")
	configExts      = []string{".json"}
)

func Config() Configuration {
	return configuration.Load().(Configuration)
}

func Load() error {
	ConfigPath = filepath.Clean(ConfigPath)
	m, _, err := ReadConfigFile(ConfigPath)
	configuration.Store(m)
	return err
}

func ReadConfigFile(filename string) (config Configuration, path string, err error) {
	config = DefaultConfig

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
