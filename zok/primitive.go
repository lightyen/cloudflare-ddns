package zok

import (
	"encoding/json"
	"strconv"
)

type Bool bool
type Integer int
type String string

func IsTrueValue(v string) bool {
	return v == "1" || v == "yes" || v == "on" || v == "true" || v == "enabled"
}

func IsFalseValue(v string) bool {
	return v == "0" || v == "no" || v == "off" || v == "false" || v == "disabled"
}

func (b Bool) Value() bool {
	return bool(b)
}

func (b *Bool) String() string {
	if b == nil {
		return strconv.FormatBool(false)
	}
	return strconv.FormatBool(b.Value())
}

func NewBool(b bool) *Bool {
	ret := Bool(b)
	return &ret
}

func (b *Bool) UnmarshalJSON(data []byte) error {
	value := string(data)
	if v, err := strconv.Unquote(value); err == nil {
		value = v
	}
	*b = Bool(IsTrueValue(value))
	return nil
}

func (b Bool) MarshalJSON() ([]byte, error) {
	return json.Marshal(bool(b))
}

func NewInteger(v int) *Integer {
	ret := Integer(v)
	return &ret
}

func (i Integer) Value() int {
	return int(i)
}

func (i *Integer) String() string {
	if i == nil {
		return "0"
	}
	return strconv.Itoa(i.Value())
}

func (i *Integer) UnmarshalJSON(data []byte) error {
	value := string(data)
	if v, err := strconv.Unquote(value); err == nil {
		value = v
	}
	val, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	*i = Integer(val)
	return nil
}

func (i Integer) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(i))
}

func (s String) Value() string {
	return string(s)
}

func (s *String) String() string {
	if s == nil {
		return ""
	}
	return s.Value()
}

func NewString(str string) *String {
	ret := String(str)
	return &ret
}

func (s *String) UnmarshalJSON(data []byte) error {
	value := string(data)
	if v, err := strconv.Unquote(value); err == nil {
		value = string(v)
	}
	*s = String(value)
	return nil
}

func (s String) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

func NewStringSlice(values []string) []String {
	s := make([]String, len(values))
	for i := range values {
		s[i] = String(values[i])
	}
	return s
}
