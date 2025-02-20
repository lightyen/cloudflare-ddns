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

func FlagParse() (err error) {
	f := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	m := Config()

	constructFlags(f, &m)

	f.Usage = func() {
		if f.Name() == "" {
			fmt.Fprintf(f.Output(), "Usage:\n")
		} else {
			fmt.Fprintf(f.Output(), "Usage of %s:\n", f.Name())
		}
		PrintDefaults(f)
	}

	err = f.Parse(os.Args[1:])

	if err == nil {
		configuration.Store(m)
	}
	return
}

func jsonTagName(f reflect.StructField) string {
	k := f.Tag.Get("json")
	index := strings.Index(k, ",")
	if index < 0 {
		return strings.TrimSpace(k)
	}
	return strings.TrimSpace(k[0:index])
}

func env(v reflect.Value, name string) {
	k := strings.ToUpper(strings.Replace(name, "-", "_", -1))
	e, exists := os.LookupEnv(k)
	if !exists {
		return
	}
	switch v.Kind().String() {
	default:

	case "int":
		if val, err := strconv.Atoi(e); err == nil {
			v.Set(reflect.ValueOf(val))
		}
	}
}

type Value interface {
	String() string
	Set(string) (err error)
	DefaultValue() string
	Type() string
}

type value struct {
	sf  reflect.Value
	def reflect.Value
}

func (i *value) Set(s string) (err error) {
	var v any

	switch i.sf.Kind().String() {
	default:
		err = errors.ErrUnsupported
	case "string":
		v = s
	case "int":
		v, err = strconv.Atoi(s)
	}

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
	return
}

func (i *value) DefaultValue() string {
	if i.def.IsValid() && i.def.CanInterface() {
		return fmt.Sprint(i.def.Interface())
	}
	return ""
}

func (i *value) Type() string {
	return i.sf.Type().String()
}

func (i *value) String() string {
	return i.DefaultValue()
}

func constructFlags(flagSet *flag.FlagSet, conf *Configuration) {
	t := reflect.TypeOf(conf).Elem()
	v := reflect.ValueOf(conf).Elem()
	d := reflect.ValueOf(&DefaultConfig).Elem()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		sf := v.Field(i)
		def := d.Field(i)
		name := jsonTagName(f)
		usage := f.Tag.Get("usage")

		// env := strings.ToUpper(strings.Replace(name, "-", "_", -1))
		switch f.Type.Kind().String() {
		default:
			// set env
			// flag.StringVar(any(ptr).(*string), name, any(value).(string), usage)
		case "string":
		case "int":
			if sf.CanSet() {
				flagSet.Var(&value{sf: sf, def: def}, name, usage)
				env(sf, name)
			}
		}
	}
}

func PrintDefaults(f *flag.FlagSet) {
	f.VisitAll(func(flag *flag.Flag) {
		var b strings.Builder
		fmt.Fprintf(&b, "  -%s", flag.Name) // Two spaces before -; see next two comments.
		usage := flag.Usage

		val := flag.Value.(Value)
		typ := val.Type()

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

		// Print the default value only if it differs to the zero value
		// for this flag type.
		fmt.Fprintf(&b, " (default %v)", val.DefaultValue())
		fmt.Fprint(f.Output(), b.String(), "\n")
	})
}
