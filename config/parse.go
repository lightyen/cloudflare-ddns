package config

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var (
	mu              sync.RWMutex
	configuration   Configuration
	ErrRecordFormat = errors.New("wrong record format")
	configExts      = []string{".json"}
)

func Config() Configuration {
	mu.RLock()
	defer mu.RUnlock()
	return configuration
}

func Load() error {
	ConfigPath = filepath.Clean(ConfigPath)
	m, _, err := ReadConfigFile(ConfigPath)
	mu.Lock()
	defer mu.Unlock()
	configuration = m
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

func FlagParse() (err error) {
	s := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	mu.Lock()
	defer mu.Unlock()

	// TODO: handle flags
	v := configuration
	s.IntVar(&v.ServePort, "http", v.ServePort, "http port")

	err = s.Parse(os.Args[1:])
	if err == nil {
		configuration = v
	}
	return
}
