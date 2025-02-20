package config

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"
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

func parseValue(f reflect.Value, s string) (v any, err error) {
	switch f.Kind().String() {
	default:
		err = errors.ErrUnsupported
	case "string":
		v = s
	case "bool":
		v, err = strconv.ParseBool(s)
	case "int":
		v, err = strconv.Atoi(s)
	case "int8":
		var n int64
		n, err = strconv.ParseInt(s, 0, 8)
		v = int8(n)
	case "int16":
		var n int64
		n, err = strconv.ParseInt(s, 0, 16)
		v = int16(n)
	case "int32":
		var n int64
		n, err = strconv.ParseInt(s, 0, 32)
		v = int32(n)
	case "int64":
		v, err = strconv.ParseInt(s, 0, 64)
	case "uint":
	case "uint8":
		var n uint64
		n, err = strconv.ParseUint(s, 0, 8)
		v = uint8(n)
	case "uint16":
		var n uint64
		n, err = strconv.ParseUint(s, 0, 16)
		v = uint16(n)
	case "uint32":
		var n uint64
		n, err = strconv.ParseUint(s, 0, 32)
		v = uint32(n)
	case "uint64":
		v, err = strconv.ParseUint(s, 0, 64)
	case "float32":
		var n float64
		n, err = strconv.ParseFloat(s, 32)
		v = float32(n)
	case "float64":
		v, err = strconv.ParseFloat(s, 64)
	case "time.Duration":
		v, err = time.ParseDuration(s)
	}
	return
}
