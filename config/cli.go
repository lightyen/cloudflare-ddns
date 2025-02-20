package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

func FlagParse() error {
	f := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	f.Usage = func() {
		if f.Name() == "" {
			fmt.Fprintf(f.Output(), "Usage:\n")
		} else {
			fmt.Fprintf(f.Output(), "Usage of %s:\n", f.Name())
		}
		PrintDefaults(f)
	}

	f.BoolVar(&PrintVersion, "v", false, "print version")
	f.BoolVar(&PrintVersion, "version", false, "print version")

	m := Config()
	if err := loadEnvFlags(f, &m); err != nil {
		return err
	}

	if err := f.Parse(os.Args[1:]); err != nil {
		return err
	}

	configuration.Store(m)
	return nil
}

func jsonTagName(f reflect.StructField) string {
	k := f.Tag.Get("json")
	index := strings.Index(k, ",")
	if index < 0 {
		return strings.TrimSpace(k)
	}
	return strings.TrimSpace(k[0:index])
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

type Value interface {
	String() string
	Set(string) (err error)
	TypeInfo() string
	DefaultValue() string
}

var _ Value = &value{}

type value struct {
	sf  reflect.Value
	def reflect.Value
}

func (i *value) Set(s string) error {
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

func (i *value) String() string {
	return i.DefaultValue()
}

func (i *value) TypeInfo() string {
	return i.sf.Type().String()
}

func (i *value) DefaultValue() string {
	if i.def.IsValid() && !i.def.IsZero() && i.def.CanInterface() {
		v := i.def.Interface()
		if s, ok := v.(string); ok {
			return strconv.Quote(s)
		}
		return fmt.Sprint(v)
	}
	return ""
}

func loadEnvFlags(flagSet *flag.FlagSet, conf *Configuration) error {
	t := reflect.TypeOf(conf).Elem()
	v := reflect.ValueOf(conf).Elem()
	d := reflect.ValueOf(&DefaultConfig).Elem()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		sf := v.Field(i)
		def := d.Field(i)
		name := jsonTagName(f)
		usage := f.Tag.Get("usage")
		if sf.CanSet() {
			if err := env(sf, name); err != nil {
				return err
			}
			flagSet.Var(&value{sf: sf, def: def}, name, usage)
		}
	}

	return nil
}

func PrintDefaults(f *flag.FlagSet) {
	f.VisitAll(func(flag *flag.Flag) {
		val, ok := flag.Value.(Value)
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
