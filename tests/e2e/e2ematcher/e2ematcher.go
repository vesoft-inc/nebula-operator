package e2ematcher

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"k8s.io/klog/v2"
)

func Struct(actual any, matchers map[string]any) (retErr error) {
	return structImpl(nil, actual, matchers)
}

func structImpl(preFields []string, actual any, matchers map[string]any) (retErr error) {
	originalActualVal := reflect.ValueOf(actual)
	for originalActualVal.Kind() == reflect.Ptr {
		originalActualVal = originalActualVal.Elem()
	}

	for field, matcher := range matchers {
		actualVal := originalActualVal
		currFields := append(preFields, field)

		var err error
		actualVal, err = extractValue(actualVal, field)
		if err != nil {
			retErr = multierror.Append(retErr, fmt.Errorf("field %q is invalid %w", currFields, err))
			continue
		}

		if !actualVal.IsValid() {
			retErr = multierror.Append(retErr, fmt.Errorf("fields %q is invalid", currFields))
			continue
		}
		klog.V(6).InfoS("e2ematcher struct impl on fields", "fields", currFields)

		if m, ok := matcher.(Matcher); ok {
			for actualVal.Kind() == reflect.Ptr {
				actualVal = actualVal.Elem()
			}
			val := actualVal.Interface()
			if err = m.Match(val); err != nil {
				retErr = multierror.Append(retErr, fmt.Errorf("fields %q(%v) does not match %w", currFields, val, err))
			}
		} else if subMatchers, ok := matcher.(map[string]any); ok {
			err = structImpl(currFields, actualVal.Interface(), subMatchers)
			if err != nil {
				retErr = multierror.Append(retErr, err)
			}
		} else {
			retErr = multierror.Append(retErr, fmt.Errorf("fields %q is invalid matcher", currFields))
		}
	}

	return retErr
}

func extractValue(val reflect.Value, field string) (reflect.Value, error) {
	switch val.Kind() {
	case reflect.Struct:
		return val.FieldByName(field), nil
	case reflect.Map:
		var keyVal reflect.Value
		switch val.Type().Key().Kind() { // key type kind
		case reflect.String:
			keyVal = reflect.ValueOf(field)
		case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
			key, err := strconv.ParseInt(field, 10, 64)
			if err != nil {
				return reflect.Value{}, err
			}
			keyVal = reflect.New(val.Type().Key()).Elem()
			keyVal.SetInt(key)
		case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
			key, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				return reflect.Value{}, err
			}
			keyVal = reflect.New(val.Type().Key()).Elem()
			keyVal.SetUint(key)
		default:
			return reflect.Value{}, fmt.Errorf("unsupported map key type %q", val.Type())
		}
		return val.MapIndex(keyVal), nil
	case reflect.Slice:
		idx, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return val.Index(int(idx)), nil
	}
	return reflect.Value{}, fmt.Errorf("unsupported value type %q", val.Type())
}
