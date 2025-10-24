package app

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/bhmj/goblocks/conftool"
)

var (
	errConfigNotStruct  = errors.New("config must be a struct or pointer to struct")
	errCopyMustBeStruct = errors.New("copySameType: both must be struct or pointer to struct")
)

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
