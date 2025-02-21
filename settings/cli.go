package settings

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
)

var (
	ErrShowVersion = errors.New("show version")
	ErrHelp        = flag.ErrHelp
	LogLevel       zapcore.Level
	printVersion   bool
)

func FlagParse() error {
	f := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	f.Usage = func() {
		if f.Name() == "" {
			fmt.Fprintf(f.Output(), "Usage:\n")
		} else {
			fmt.Fprintf(f.Output(), "Usage of %s:\n", f.Name())
		}
		printDefaults(f)
	}

	f.Var(&loglevel{}, "log-level", "the level of log messages (debug|info|warn|error|dpanic|panic|fatal)")
	f.Var(&versionValue{}, "v", "print version")
	f.Var(&versionValue{}, "version", "print version")

	m := *Value()
	if err := loadEnvFlags(f, &m); err != nil {
		return err
	}

	if v, exists := os.LookupEnv("LOG_LEVEL"); exists {
		if err := LogLevel.Set(v); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
	}

	if err := f.Parse(os.Args[1:]); err != nil {
		return err
	}

	if printVersion {
		fmt.Fprintln(os.Stdout, Version)
		return ErrShowVersion
	}

	value.Store(&m)
	return nil
}

func structTag(f reflect.StructField, key string) (string, []string) {
	s := strings.Split(f.Tag.Get(key), ",")
	for i := range s {
		s[i] = strings.TrimSpace(s[i])
	}
	if len(s) > 0 {
		return s[0], s[1:]
	}
	return "", nil
}

func env(f reflect.Value, name string) error {
	k := strings.ToUpper(strings.Replace(name, "-", "_", -1))
	s, exists := os.LookupEnv(k)
	if !exists {
		return nil
	}

	v, err := parseValue(f, s)
	if err == nil {
		f.Set(reflect.ValueOf(v))
	}

	return err
}

func loadEnvFlags(flagSet *flag.FlagSet, conf *Settings) error {
	t := reflect.TypeOf(conf).Elem()
	v := reflect.ValueOf(conf).Elem()
	d := reflect.ValueOf(&Default).Elem()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		sf := v.Field(i)
		def := d.Field(i)
		usage, _ := structTag(f, "usage")
		jsonKey, _ := structTag(f, "json")
		cli, cliOptions := structTag(f, "cli")

		if slices.Contains(cliOptions, "ignored") {
			continue
		}

		if jsonKey == "" {
			continue
		}

		name := cli
		if name == "" {
			name = strings.Replace(jsonKey, "_", "-", -1)
		}

		if name == "" {
			continue
		}

		if sf.CanSet() {
			if err := env(sf, name); err != nil {
				return err
			}
			flagSet.Var(&anyValue{sf: sf, def: def}, name, usage)
		}
	}

	return nil
}

func printDefaults(f *flag.FlagSet) {
	f.VisitAll(func(flag *flag.Flag) {
		val, ok := flag.Value.(iValue)
		if !ok {
			return
		}

		var b strings.Builder
		fmt.Fprintf(&b, "  -%s", flag.Name) // Two spaces before -; see next two comments.
		usage := flag.Usage

		typ := val.TypeInfo()

		// name, usage := UnquoteUsage(flag)
		if len(typ) > 0 {
			b.WriteString(" ")
			b.WriteString(typ)
		}

		// Boolean flags of one ASCII letter are so common we
		// treat them specially, putting their usage on the same line.
		if b.Len() <= 4 { // space, space, '-', 'x'.
			b.WriteString("\t")
		} else {
			// Four spaces before the tab triggers good alignment
			// for both 4- and 8-space tab stops.
			b.WriteString("\n    \t")
		}
		b.WriteString(strings.ReplaceAll(usage, "\n", "\n    \t"))

		defaultValue := val.DefaultValue()
		if defaultValue != "" {
			fmt.Fprintf(&b, " (default %v)", defaultValue)
		}
		fmt.Fprint(f.Output(), b.String(), "\n")
	})
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
		strconv.ParseUint(s, 0, strconv.IntSize)
		var n uint64
		n, err = strconv.ParseUint(s, 0, 8)
		v = uint(n)
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

type iValue interface {
	String() string
	Set(string) (err error)
	TypeInfo() string
	DefaultValue() string
}

var _ iValue = &anyValue{}

type anyValue struct {
	sf  reflect.Value
	def reflect.Value
}

func (i *anyValue) Set(s string) error {
	v, err := parseValue(i.sf, s)
	if err != nil {
		if errors.Is(err, strconv.ErrSyntax) {
			return strconv.ErrSyntax
		}
		if errors.Is(err, strconv.ErrRange) {
			return strconv.ErrRange
		}
		return err
	}
	i.sf.Set(reflect.ValueOf(v))
	return nil
}

func (i *anyValue) String() string {
	return i.DefaultValue()
}

func (i *anyValue) IsBoolFlag() bool {
	return i.sf.Type().Kind() == reflect.Bool
}

func (i *anyValue) TypeInfo() string {
	return i.sf.Type().String()
}

func (i *anyValue) DefaultValue() string {
	if i.def.IsValid() && !i.def.IsZero() && i.def.CanInterface() {
		v := i.def.Interface()
		if s, ok := v.(string); ok {
			return strconv.Quote(s)
		}
		return fmt.Sprint(v)
	}
	return ""
}

type loglevel struct {
}

func (v *loglevel) Set(s string) error {
	return LogLevel.Set(s)
}

func (v *loglevel) String() string {
	return v.DefaultValue()
}

func (v *loglevel) TypeInfo() string {
	return reflect.TypeOf(LogLevel).String()
}

func (v *loglevel) DefaultValue() string {
	return ""
}

type versionValue struct {
}

func (i *versionValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		if errors.Is(err, strconv.ErrSyntax) {
			return strconv.ErrSyntax
		}
		if errors.Is(err, strconv.ErrRange) {
			return strconv.ErrRange
		}
		return err
	}
	if !printVersion {
		printVersion = v
	}
	return nil
}

func (i *versionValue) String() string {
	return i.DefaultValue()
}

func (i *versionValue) IsBoolFlag() bool {
	return true
}

func (i *versionValue) TypeInfo() string {
	return "bool"
}

func (i *versionValue) DefaultValue() string {
	return "false"
}
