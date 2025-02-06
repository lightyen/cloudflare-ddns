package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
)

var (
	configPath      = "config.json"
	ErrRecordFormat = errors.New("wrong record format")
)

func Parse() error {
	m, t := readConfigFile(configPath)
	return parse(m, t, &Config)
}

var configExts = []string{".json"}

func readConfigFile(filename string) (config map[string]any, path string) {
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

		buf := make([]byte, 2<<10)
		n, err := f.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			continue
		}

		cfg := make(map[string]any)

		switch ext {
		case ".json":
			if err := json.Unmarshal(buf[:n], &cfg); err == nil {
				return cfg, target
			}
		}
	}

	return make(map[string]any), ""
}

func jsonTagKey(t reflect.StructTag) string {
	f := strings.SplitN(t.Get("json"), ",", 2)
	return strings.TrimSpace(f[0])
}

func parse(config map[string]any, configPath string, val any) error {
	flagValues := map[string]any{}

	t, s := reflect.TypeOf(val).Elem(), reflect.ValueOf(val).Elem()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		name := jsonTagKey(field.Tag)
		usage := field.Tag.Get("usage")
		defaultValue := field.Tag.Get("default")

		switch field.Type.String() {
		default:
			return fmt.Errorf("Field %q type %s is not handled", name, field.Type.String())
		case "[]config.Record":
			// read config only
			var v []Record
			if s, exists := config[name]; exists {
				arr, ok := s.([]any)
				if !ok {
					return ErrRecordFormat
				}
				for _, m := range arr {
					data, ok := m.(map[string]any)
					if !ok {
						return ErrRecordFormat
					}
					name, _ := (data["name"]).(string)
					typ, _ := (data["type"]).(string)
					var proxied bool
					vProxied, exists := data["proxied"]
					if exists {
						proxied, ok = (vProxied).(bool)
						if !ok {
							return ErrRecordFormat
						}
					}
					if name == "" || (typ != "A" && typ != "AAAA") {
						return ErrRecordFormat
					}
					v = append(v, Record{Name: name, Type: typ, Proxied: proxied})
				}
				flagValues[name] = &v
			}

		case "string":
			var v string
			var d = defaultValue

			if s, exists := config[name]; exists {
				if x, ok := s.(string); ok {
					d = x
				} else {
					return fmt.Errorf("config %q is %s, shound be %s", name, reflect.TypeOf(s).String(), field.Type.String())
				}
			}

			flagValues[name] = &v
			flag.StringVar(&v, name, d, usage)
		case "bool":
			var v bool
			d, err := strconv.ParseBool(defaultValue)
			if err != nil {
				return fmt.Errorf("config %q defaultValue: %w", name, err)
			}

			if s, exists := config[name]; exists {
				if x, ok := s.(bool); ok {
					d = x
				} else {
					return fmt.Errorf("config %q is %s, shound be %s", name, reflect.TypeOf(s).String(), field.Type.String())
				}
			}

			flagValues[name] = &v
			flag.BoolVar(&v, name, d, usage)
		case "int64":
			var v int64
			d, err := strconv.ParseInt(defaultValue, 0, 64)
			if err != nil {
				return fmt.Errorf("config %q defaultValue: %w", name, err)
			}

			if s, exists := config[name]; exists {
				if x, ok := config[name].(int64); ok {
					d = int64(x)
				} else {
					return fmt.Errorf("config %q is %s, shound be %s", name, reflect.TypeOf(s).String(), field.Type.String())
				}
			}

			flagValues[name] = &v
			flag.Int64Var(&v, name, d, usage)
		case "uint64":
			var v uint64
			d, err := strconv.ParseUint(defaultValue, 0, 64)
			if err != nil {
				return fmt.Errorf("config %q defaultValue: %w", name, err)
			}

			if s, exists := config[name]; exists {
				if x, ok := config[name].(uint64); ok {
					d = x
				} else {
					return fmt.Errorf("config %q is %s, shound be %s", name, reflect.TypeOf(s).String(), field.Type.String())
				}
			}

			flagValues[name] = &v
			flag.Uint64Var(&v, name, d, usage)
		case "int":
			var v int
			d, err := strconv.Atoi(defaultValue)
			if err != nil {
				return fmt.Errorf("config %q defaultValue: %w", name, err)
			}

			if s, exists := config[name]; exists {
				if x, ok := config[name].(int); ok {
					d = x
				} else {
					return fmt.Errorf("config %q is %s, shound be %s", name, reflect.TypeOf(s).String(), field.Type.String())
				}
			}

			flagValues[name] = &v
			flag.IntVar(&v, name, d, usage)
		case "uint":
			var v uint
			dd, err := strconv.Atoi(defaultValue)
			if err != nil {
				return fmt.Errorf("config %q defaultValue: %w", name, err)
			}
			d := uint(dd)

			if s, exists := config[name]; exists {
				if x, ok := config[name].(uint); ok {
					d = x
				} else {
					return fmt.Errorf("config %q is %s, shound be %s", name, reflect.TypeOf(s).String(), field.Type.String())
				}
			}

			flagValues[name] = &v
			flag.UintVar(&v, name, d, usage)
		case "float64":
			var v float64
			d, err := strconv.ParseFloat(defaultValue, 64)
			if err != nil {
				return fmt.Errorf("config %q defaultValue: %w", name, err)
			}

			if s, exists := config[name]; exists {
				if x, ok := config[name].(float64); ok {
					d = x
				} else {
					return fmt.Errorf("config %q is %s, shound be %s", name, reflect.TypeOf(s).String(), field.Type.String())
				}
			}

			flagValues[name] = &v
			flag.Float64Var(&v, name, d, usage)
		case "zapcore.Level":
			var v zapcore.Level
			if err := v.Set(defaultValue); err != nil {
				return fmt.Errorf("config %q defaultValue: %w", name, err)
			}

			if s, exists := config[name]; exists {
				if x, ok := config[name].(string); ok {
					if err := v.Set(x); err != nil {
						return fmt.Errorf("%s %q: %w", configPath, name, err)
					}
				} else {
					return fmt.Errorf("config %q is %s, shound be %s", name, reflect.TypeOf(s).String(), field.Type.String())
				}
			}

			flagValues[name] = &v
			flag.Var(&v, name, usage)
		case "time.Duration":
			var v time.Duration
			d, err := time.ParseDuration(defaultValue)
			if err != nil {
				return fmt.Errorf("config %q defaultValue: %w", name, err)
			}

			if s, exists := config[name]; exists {
				if x, ok := config[name].(string); ok {
					x, err := time.ParseDuration(x)
					if err != nil {
						return fmt.Errorf("%s %q: %w", configPath, name, err)
					}
					d = x
				} else {
					return fmt.Errorf("config %q is %s, shound be %s", name, reflect.TypeOf(s).String(), field.Type.String())
				}
			}

			flagValues[name] = &v
			flag.DurationVar(&v, name, d, usage)
		}
	}

	flag.Parse()

	for i := 0; i < s.NumField(); i++ {
		sf := s.Field(i)
		tf := t.Field(i)
		name := jsonTagKey(tf.Tag)
		if v, exists := flagValues[name]; exists {
			if sf.CanSet() {
				v := reflect.ValueOf(v)
				if !v.IsNil() {
					v = v.Elem()
					if tf.Type.String() == v.Type().String() {
						sf.Set(v)
					} else {
						return fmt.Errorf("Field %q another type %s is not handled", name, tf.Type.String())
					}
				}
			}
		}
	}

	return nil
}
