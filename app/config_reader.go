package app

import (
	"errors"
	"flag"
	"fmt"
	"log"
	syslog "log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/bhmj/goblocks/conftool"
	"gopkg.in/yaml.v3"
)

var (
	errConfigNotStruct  = errors.New("config must be a struct or pointer to struct")
	errCopyMustBeStruct = errors.New("copySameType: both must be struct or pointer to struct")
)

// readConfigStruct reads config from the provided struct
func (a *application) readConfigStruct(config any) {
	pwd, err := os.Getwd()
	if err != nil {
		syslog.Fatalf("get current dir: %s", err)
	}
	a.cfgPath = pwd

	err = a.applyConfigStruct(config)
	if err != nil {
		syslog.Fatalf("read config data: %s", err)
	}
}

// readConfigFile reads config from the file specified by `--config-file`
func (a *application) readConfigFile() {
	pstr := flag.String("config-file", "", "Application YAML config file")
	print := flag.String("print-config", "", "Print complete config")
	flag.Parse()
	if pstr == nil || *pstr == "" {
		syslog.Fatalf("Usage: %s --config-file=/path/to/config.yaml", os.Args[0])
	}
	a.cfgPath = filepath.Dir(*pstr)

	fullname, err := filepath.Abs(*pstr)
	if err != nil {
		syslog.Fatalf("filepath: %s", err)
	}
	raw, err := os.ReadFile(fullname)
	if err != nil {
		syslog.Fatalf("read config: %s", err)
	}

	data := conftool.ParseEnvVars(raw)
	err = a.readConfigData(data)
	if err != nil {
		syslog.Fatalf("read config data: %s", err)
	}
	if print != nil {
		a.printConfig()
	}
}

func (a *application) readConfigData(data []byte) error {
	var root yaml.Node

	cfg := make(map[string]any)

	for name, reg := range a.serviceDefs {
		cfg[name] = reg.Config
	}

	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}

	if len(root.Content) == 0 {
		return errEmptyConfig
	}

	mapping := root.Content[0] // top-level mapping
	rootNodes := make(map[string]*yaml.Node)
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i].Value
		val := mapping.Content[i+1]
		rootNodes[key] = val
	}

	// decode app config
	if node, ok := rootNodes["app"]; ok {
		if err := node.Decode(a.cfg); err != nil {
			return fmt.Errorf("app config: %w", err)
		}
	}
	if err := conftool.DefaultsAndRequired(a.cfg); err != nil {
		return fmt.Errorf("app config: missing required value: %w", err)
	}

	// decode configs for all registered services
	for name, service := range a.serviceDefs {
		node, ok := rootNodes[name]
		if !ok {
			continue
		}
		if err := node.Decode(service.Config); err != nil {
			return fmt.Errorf("decode %s: %w", name, err)
		}
	}
	for name, service := range a.serviceDefs {
		if err := conftool.DefaultsAndRequired(service.Config); err != nil {
			return fmt.Errorf("%s config: missing required value: %w", name, err)
		}
	}

	return nil
}

// applyConfigStruct copies matching subconfigs (by yaml tag) from src into the
// application’s own config (a.cfg) and registered service configs.
func (a *application) applyConfigStruct(src any) error {
	v := reflect.ValueOf(src)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return errConfigNotStruct
	}

	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		yamlKey := yamlTag(field)
		if yamlKey == "" {
			continue
		}

		fv := v.Field(i)
		if !fv.CanInterface() {
			continue
		}

		switch yamlKey {
		case "app":
			err := copySameType(a.cfg, fv.Interface())
			if err != nil {
				return fmt.Errorf("copy app config: %w", err)
			}
			if err := conftool.DefaultsAndRequired(a.cfg); err != nil {
				return fmt.Errorf("required field not set in %s section: %w", "app", err)
			}

		default:
			if service, ok := a.serviceDefs[yamlKey]; ok {
				err := copySameType(service.Config, fv.Interface())
				if err != nil {
					return fmt.Errorf("copy app config: %w", err)
				}
				if err := conftool.DefaultsAndRequired(service.Config); err != nil {
					return fmt.Errorf("required field not set in %s section: %w", service.Name, err)
				}
			}
		}
	}

	return nil
}

// yamlTag extracts the main yaml tag (before comma options).
func yamlTag(f reflect.StructField) string {
	tag := f.Tag.Get("yaml")
	if tag == "" {
		return ""
	}
	return strings.Split(tag, ",")[0]
}

func copySameType(dst, src any) error {
	dv := reflect.ValueOf(dst)
	sv := reflect.ValueOf(src)

	// Dereference pointers
	if dv.Kind() == reflect.Pointer {
		dv = dv.Elem()
	}
	if sv.Kind() == reflect.Pointer {
		sv = sv.Elem()
	}

	if dv.Kind() != reflect.Struct || sv.Kind() != reflect.Struct {
		return errCopyMustBeStruct
	}
	if dv.Type() != sv.Type() {
		return fmt.Errorf("copySameType: type mismatch (%s vs %s)", dv.Type(), sv.Type())
	}

	for i := range dv.NumField() {
		df := dv.Field(i)
		sf := sv.Field(i)
		if df.CanSet() {
			df.Set(sf)
		}
	}
	return nil
}

func (a *application) printConfig() {
	allConfigs := make(map[string]any)
	allConfigs["app"] = a.cfg
	for i := range a.serviceDefs {
		allConfigs[a.serviceDefs[i].Name] = a.serviceDefs[i].Config
	}

	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	defer encoder.Close()

	if err := encoder.Encode(allConfigs); err != nil {
		log.Fatalf("error encoding YAML: %v", err)
	}
}
