package pipeline

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// ParamSpec describes one parameter of a method, for `heatmap list` and the web UI.
type ParamSpec struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // int, float, bool, string, points
	Default any    `json:"default"`
	Help    string `json:"help"`
}

// Params gives typed access to the `params:` map of a stage config.
//
// Each getter takes the parameter name, its default and a one-line help text.
// Methods call the getters while building their stage, which both reads the
// values and documents them: Describe builds a method with no parameters and
// records the getter calls, so defaults live in exactly one place.
//
// Errors (wrong type, unknown key) are collected and returned by Err.
type Params struct {
	raw   map[string]any
	used  map[string]bool
	specs []ParamSpec
	errs  []error
}

// NewParams wraps a raw parameter map (nil is fine).
func NewParams(raw map[string]any) *Params {
	return &Params{raw: raw, used: map[string]bool{}}
}

func (p *Params) get(name, typ string, def any, help string) (any, bool) {
	p.specs = append(p.specs, ParamSpec{Name: name, Type: typ, Default: def, Help: help})
	p.used[name] = true
	v, ok := p.raw[name]
	return v, ok && v != nil
}

func (p *Params) fail(name, want string, v any) {
	p.errs = append(p.errs, fmt.Errorf("parameter %q: want %s, got %v (%T)", name, want, v, v))
}

// Int returns an integer parameter. Whole floats such as 5.0 are accepted.
func (p *Params) Int(name string, def int, help string) int {
	v, ok := p.get(name, "int", def, help)
	if !ok {
		return def
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1<<31 {
			return int(x)
		}
	}
	p.fail(name, "an integer", v)
	return def
}

// Float returns a number parameter. NaN and infinities are rejected.
func (p *Params) Float(name string, def float64, help string) float64 {
	v, ok := p.get(name, "float", def, help)
	if !ok {
		return def
	}
	var f float64
	switch x := v.(type) {
	case float64:
		f = x
	case int:
		f = float64(x)
	case int64:
		f = float64(x)
	default:
		p.fail(name, "a number", v)
		return def
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		p.fail(name, "a finite number", v)
		return def
	}
	return f
}

// Float32 is Float converted to float32.
func (p *Params) Float32(name string, def float32, help string) float32 {
	// Go through the shortest decimal form so a default of 0.05 is reported
	// as 0.05, not as 0.05000000074505806.
	d, _ := strconv.ParseFloat(strconv.FormatFloat(float64(def), 'g', -1, 32), 64)
	return float32(p.Float(name, d, help))
}

// Bool returns a boolean parameter.
func (p *Params) Bool(name string, def bool, help string) bool {
	v, ok := p.get(name, "bool", def, help)
	if !ok {
		return def
	}
	b, isBool := v.(bool)
	if !isBool {
		p.fail(name, "true or false", v)
		return def
	}
	return b
}

// String returns a string parameter.
func (p *Params) String(name, def, help string) string {
	v, ok := p.get(name, "string", def, help)
	if !ok {
		return def
	}
	s, isStr := v.(string)
	if !isStr {
		p.fail(name, "a string", v)
		return def
	}
	return s
}

// Points returns a list of [x, y] pairs.
func (p *Params) Points(name string, def [][2]float64, help string) [][2]float64 {
	v, ok := p.get(name, "points", def, help)
	if !ok {
		return def
	}
	list, isList := v.([]any)
	if !isList {
		p.fail(name, "a list of [x, y] pairs", v)
		return def
	}
	out := make([][2]float64, 0, len(list))
	for _, item := range list {
		pair, isPair := item.([]any)
		if !isPair || len(pair) != 2 {
			p.fail(name, "a list of [x, y] pairs", v)
			return def
		}
		var xy [2]float64
		for k, c := range pair {
			switch n := c.(type) {
			case float64:
				xy[k] = n
			case int:
				xy[k] = float64(n)
			default:
				p.fail(name, "numeric coordinates", v)
				return def
			}
		}
		out = append(out, xy)
	}
	return out
}

// Errorf records a validation error found by the method itself.
func (p *Params) Errorf(format string, args ...any) {
	p.errs = append(p.errs, fmt.Errorf(format, args...))
}

// Err returns all collected errors, including keys that no getter asked for
// (typos such as "sigam").
func (p *Params) Err() error {
	var unknown []string
	for k := range p.raw {
		if !p.used[k] {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	errs := p.errs
	if len(unknown) > 0 {
		var known []string
		for _, s := range p.specs {
			known = append(known, s.Name)
		}
		errs = append(errs, fmt.Errorf("unknown parameter(s) %s (known: %s)",
			strings.Join(unknown, ", "), strings.Join(known, ", ")))
	}
	return errors.Join(errs...)
}

// Specs returns the parameters requested so far, in call order.
func (p *Params) Specs() []ParamSpec {
	return p.specs
}
