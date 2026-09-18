package artifacts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

func failure(path, reason string) error { return fmt.Errorf("%s: %s", path, reason) }

// Decode accepts only caller-supplied bytes, with no external I/O. On every
// failure it returns a nil catalog and a bounded, source-text-free diagnostic.
// json.Marshal preserves the resulting arrays and optional field presence.
func Decode(data []byte, limits Limits) (*Catalog, error) {
	limits, err := normalizeLimits(limits)
	if err != nil {
		return nil, err
	}
	if len(data) > limits.Bytes {
		return nil, failure("$", "byte limit exceeded")
	}
	if !utf8.Valid(data) {
		return nil, failure("$", "invalid UTF-8")
	}
	if err := checkEscapes(data); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := scan(d, 0, limits.Depth); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, failure("$", "trailing JSON or invalid syntax")
	}
	// Reflection is restricted to this package's fixed wire types. It enforces
	// exact (case-sensitive) member names and requiredness before encoding/json,
	// which otherwise accepts case aliases and absent zero-valued fields.
	if err := checkShape(data, reflect.TypeOf(Catalog{}), "$"); err != nil {
		return nil, err
	}
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, failure("$", "invalid field type")
	}
	if err := validate(&c, limits); err != nil {
		return nil, err
	}
	return &c, nil
}

func normalizeLimits(l Limits) (Limits, error) {
	c := ceilings()
	pairs := [][2]*int{{&l.Bytes, &c.Bytes}, {&l.Depth, &c.Depth}, {&l.Artifacts, &c.Artifacts},
		{&l.Relationships, &c.Relationships}, {&l.Applicability, &c.Applicability},
		{&l.Inputs, &c.Inputs}, {&l.Sources, &c.Sources}, {&l.NestedEntries, &c.NestedEntries}}
	for _, p := range pairs {
		if *p[0] < 0 || *p[0] > *p[1] {
			return l, failure("$", "invalid caller limit")
		}
		if *p[0] == 0 {
			*p[0] = *p[1]
		}
	}
	return l, nil
}

// scan checks duplicate decoded keys and depth without constructing a JSON tree.
// Syntax diagnostics deliberately omit encoding/json's source excerpts.
func scan(d *json.Decoder, depth, maxDepth int) error {
	return scanAt(d, depth, maxDepth, "$")
}

func scanAt(d *json.Decoder, depth, maxDepth int, path string) error {
	token, err := d.Token()
	if err != nil {
		return failure(path, "invalid JSON syntax")
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	depth++
	if depth > maxDepth {
		return failure(path, "depth limit exceeded")
	}
	if delim != '{' && delim != '[' {
		return failure(path, "invalid JSON syntax")
	}
	keys := map[string]bool{}
	for index := 0; d.More(); index++ {
		childPath := fmt.Sprintf("%s[%d]", path, index)
		if delim == '{' {
			childPath = fmt.Sprintf("%s{%d}", path, index)
		}
		if delim == '{' {
			key, err := d.Token()
			if err != nil {
				return failure(path, "invalid JSON member")
			}
			name, ok := key.(string)
			if !ok {
				return failure(path, "invalid JSON member")
			}
			if keys[name] {
				return failure(childPath, "duplicate JSON member")
			}
			keys[name] = true
		}
		if err := scanAt(d, depth, maxDepth, childPath); err != nil {
			return err
		}
	}
	end, err := d.Token()
	if err != nil || (delim == '{' && end != json.Delim('}')) || (delim == '[' && end != json.Delim(']')) {
		return failure(path, "invalid JSON syntax")
	}
	return nil
}

// encoding/json replaces unpaired surrogates; reject them before that decoding.
func checkEscapes(data []byte) error {
	for i := 0; i < len(data); i++ {
		if data[i] != '\\' {
			continue
		}
		i++
		if i >= len(data) {
			return failure("$", "invalid JSON escape")
		}
		if data[i] != 'u' {
			continue
		}
		if i+4 >= len(data) {
			return failure("$", "invalid Unicode escape")
		}
		n, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		if err != nil {
			return failure("$", "invalid Unicode escape")
		}
		i += 4
		if n < 0xd800 || n > 0xdfff {
			continue
		}
		if n > 0xdbff || i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
			return failure("$", "unpaired Unicode surrogate")
		}
		low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return failure("$", "unpaired Unicode surrogate")
		}
		i += 6
	}
	return nil
}

func checkShape(raw []byte, typ reflect.Type, path string) error {
	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("null")) {
		return failure(path, "null is not allowed")
	}
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return failure(path, "expected object")
		}
		allowed := map[string]bool{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")
			name := tag[0]
			allowed[name] = true
			v, exists := obj[name]
			if !exists {
				if len(tag) == 1 {
					return failure(path+"."+name, "required field missing")
				}
				continue
			}
			if err := checkShape(v, f.Type, path+"."+name); err != nil {
				return err
			}
		}
		for name := range obj {
			if !allowed[name] {
				return failure(path, "unknown field")
			}
		}
	case reflect.Slice:
		var list []json.RawMessage
		if err := json.Unmarshal(raw, &list); err != nil {
			return failure(path, "expected array")
		}
		for i, v := range list {
			if err := checkShape(v, typ.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.String:
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return failure(path, "expected string")
		}
	case reflect.Bool:
		if string(raw) != "true" && string(raw) != "false" {
			return failure(path, "expected boolean")
		}
	case reflect.Int64:
		for _, b := range raw {
			if b < '0' || b > '9' {
				return failure(path, "expected nonnegative integer notation")
			}
		}
		v, err := strconv.ParseUint(string(raw), 10, 64)
		if err != nil || v > 9007199254740991 {
			return failure(path, "integer range exceeded")
		}
	}
	return nil
}
