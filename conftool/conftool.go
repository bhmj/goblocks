package conftool

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var errConfigTypeNotSupported = errors.New("unsupported config file type")

type unmarshaller func([]byte, any) error

// ReadFromFile reads config from file.
func ReadFromFile(fname string, cfg any) error {
	fullname, err := filepath.Abs(fname)
	if err != nil {
		return fmt.Errorf("filepath.Abs: %w", err)
	}
	f, err := os.Open(fullname)
	if err != nil {
		return fmt.Errorf("open config file: %w", err)
	}
	defer f.Close()
	decoder, err := getConfigType(fname)
	if err != nil {
		return err
	}
	return parseRead(f, decoder, cfg)
}

// ParseEnvVars replaces {{VAR}} entries with respective environment values.
func ParseEnvVars(buf []byte) []byte {
	rx := regexp.MustCompile(`{{(\w+)}}`)
	for {
		matches := rx.FindSubmatch(buf)
		if matches == nil {
			break
		}
		v := os.Getenv(strings.ToUpper(string(matches[1])))
		buf = bytes.ReplaceAll(buf, matches[0], []byte(v))
	}

	return buf
}

func DefaultsAndRequired(cfg any) error {
	reqs := defsAndReqs(cfg)
	if len(reqs) > 0 {
		return fmt.Errorf("required config parameters not set: %s", strings.Join(reqs, ", ")) //nolint:err113
	}
	return nil
}

func getConfigType(fname string) (unmarshaller, error) {
	ext := filepath.Ext(fname)
	switch ext {
	case ".yaml", ".yml":
		return yaml.Unmarshal, nil
	case ".json":
		return json.Unmarshal, nil
	default:
		return nil, fmt.Errorf("%w: %s", errConfigTypeNotSupported, ext)
	}
}

func parseRead(f io.Reader, decoder unmarshaller, cfg any) error {
	conf, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("config file ReadAll: %w", err)
	}
	conf = ParseEnvVars(conf)

	if err := decoder(conf, cfg); err != nil {
		return fmt.Errorf("decoding config file: %w", err)
	}

	return DefaultsAndRequired(cfg)
}

func defsAndReqs(cfg any) []string {
	var reqs []string
	val := reflect.ValueOf(cfg).Elem()
	typ := val.Type()

	for i := range val.NumField() {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Set the default value based on the field kind
		if field.Kind() == reflect.Struct { //nolint:nestif
			// If it's a struct, recurse
			name := fieldType.Name
			dive := defsAndReqs(field.Addr().Interface())
			if len(dive) > 0 {
				for _, d := range dive {
					reqs = append(reqs, fmt.Sprintf("%s.%s", name, d))
				}
			}
		} else if field.CanSet() {
			isZeroValue := isFieldEmpty(field)
			if !isZeroValue {
				continue
			}
			// Check if the field has a `required` tag
			isRequired := isFieldRequired(fieldType)
			// Check if the field has a `default` tag
			defaultValue, hasDefault := fieldType.Tag.Lookup("default")
			if !hasDefault {
				if isRequired {
					reqs = append(reqs, fieldType.Name)
				}
				continue
			}

			setField(field, defaultValue)
		}
	}
	return reqs
}

func isFieldRequired(field reflect.StructField) bool {
	required, ok := field.Tag.Lookup("required")
	if ok && required == "true" {
		return true
	}
	return false
}

func isFieldEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Slice:
		// A slice is zero value if it's nil or has a length of 0
		return v.IsNil() || v.Len() == 0
	case reflect.Map:
		// A slice is zero value if it's nil or has a length of 0
		return v.IsNil() || v.Len() == 0
	default:
		// For other types, compare with the zero value
		return v.Interface() == reflect.Zero(v.Type()).Interface()
	}
}

func setField(field reflect.Value, defaultValue string) {
	switch field.Kind() {
	case reflect.String:
		field.SetString(defaultValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intValue, err := strconv.ParseInt(defaultValue, 10, 64)
		if err != nil {
			errSyntax := strconv.ErrSyntax
			if errors.Is(err, errSyntax) {
				dur, err := time.ParseDuration(defaultValue)
				if err == nil {
					field.SetInt(int64(dur))
				}
			}
		} else {
			field.SetInt(intValue)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if uintValue, err := strconv.ParseUint(defaultValue, 10, 64); err == nil {
			field.SetUint(uintValue)
		}
	case reflect.Float32, reflect.Float64:
		if floatValue, err := strconv.ParseFloat(defaultValue, 64); err == nil {
			field.SetFloat(floatValue)
		}
	case reflect.Bool:
		if boolValue, err := strconv.ParseBool(defaultValue); err == nil {
			field.SetBool(boolValue)
		}
	}
}
