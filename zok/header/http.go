package header

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

type AcceptSpec struct {
	Value string
	Q     float64
}

type Accepts []AcceptSpec

// Parse HTTP header: 'Accept-Language'
func ParseAccept(header string) Accepts {
	if header == "" {
		return nil
	}
	var items []AcceptSpec
	for _, v := range strings.Split(header, ",") {
		v = strings.TrimSpace(v)
		if len(v) > 0 && !strings.Contains(v, ";") {
			items = append(items, AcceptSpec{Value: v, Q: 1.000})
		} else if s := strings.Split(v, ";"); len(s) == 2 {
			spec := AcceptSpec{}
			spec.Value = strings.TrimSpace(s[0])
			spec.Q = 1.000 // default
			s[1] = strings.TrimSpace(s[1])
			if strings.HasPrefix(s[1], "q=") {
				q, err := strconv.ParseFloat(s[1][2:], 64)
				if err == nil && !math.IsNaN(q) && !math.IsInf(q, 0) && q >= 0.0 {
					if q > 1.000 {
						q = 1.000
					}
					spec.Q = q
				}
			}
			items = append(items, spec)
		}
	}
	return items
}

func (a Accepts) Contains(value string) bool {
	for _, spec := range a {
		// NOTE: 0 means not "not acceptable"
		if spec.Value == value && spec.Q >= 0.001 { // min value
			return true
		}
	}
	return false
}

func (a Accepts) ContainPrefix(value string) bool {
	for _, spec := range a {
		// NOTE: 0 means not "not acceptable"
		if strings.HasPrefix(spec.Value, value) && spec.Q >= 0.001 { // min value
			return true
		}
	}
	return false
}

type AcceptEncoding []string

func ParseAcceptEncoding(header string) AcceptEncoding {
	s := strings.Split(header, ",")
	for i := range s {
		s[i] = strings.TrimSpace(s[i])
	}
	return s
}

func (a AcceptEncoding) Contains(value string) bool {
	return slices.Contains(a, value)
}
