package gencfg

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// sanitizeTestFunctions only used by Sanitize tests and should never be invoked otherwise.
type sanitizeTestFunctions struct{}

// ReportTestCall is a test function that could be called by the sanitize function with the tag sanitize:"test_call=ReportTestCall".
// Add more test methods with the same signature to sanitizeTestFunctions as needed.
func (sanitizeTestFunctions) ReportTestCall(name, data string) error {
	fmt.Println("ReportTestCall called with", name, data)
	return nil
}

func (s sanitizeTestFunctions) invokeMethodByName(methodName, paramName, paramValue string) error {
	method := reflect.ValueOf(s).MethodByName(methodName)
	if !method.IsValid() {
		return fmt.Errorf("function '%s' not found for '%s', value='%s'", methodName, paramName, paramValue)
	}
	res := method.Call([]reflect.Value{reflect.ValueOf(paramName), reflect.ValueOf(paramValue)})
	if len(res) > 0 && !res[0].IsNil() {
		return res[0].Interface().(error)
	}
	return nil
}

// Sanitize function can be used as a simple and consistent interface for sanitizing structs,
// while the more complex logic is encapsulated within the sanitize function.
func Sanitize(inputStruct any) error {
	val := reflect.ValueOf(inputStruct)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("sanitize expected pointer to struct, got %v", val.Kind())
	}
	return sanitize(val, "root", "")
}

func sanitize(val reflect.Value, name, parentTags string) error {
	if !val.IsValid() || val.Kind() == reflect.Ptr && val.IsNil() {
		return nil
	}

	element := val.Elem()
	if element.Kind() != reflect.Struct {
		return sanitizeValue(element, name, parentTags)
	}

	for i := 0; i < element.NumField(); i++ {
		field := element.Field(i)
		fieldType := element.Type().Field(i)
		if fieldType.PkgPath != "" {
			continue // skip unexported fields
		}
		fieldKind := field.Kind()
		fieldName := fieldType.Name

		// Do not propagate tags from parent to structure fields!
		//
		// When a field is a structure assigning sanitize tags to this field is
		// pointless - its own fields must have sanitize tags if necessary.
		//
		// NOTE: more complicated logic when sanitize actions from the parent
		// structure are applied on child fields without tags or combined with actions
		// on child fields is possible, but is difficult to follow in real life.
		tags := fieldType.Tag.Get("sanitize")

		switch fieldKind {
		case reflect.Pointer:
			if err := sanitize(field, fieldName, tags); err != nil {
				return err
			}
		case reflect.Struct:
			if err := sanitize(field.Addr(), fieldName, tags); err != nil {
				return err
			}
		case reflect.Array, reflect.Slice:
			if err := sanitizeArrayOrSlice(field, fieldName, tags); err != nil {
				return err
			}
		case reflect.Map:
			if err := sanitizeMap(field, fieldName, tags); err != nil {
				return err
			}
		default:
			if err := sanitizeValue(field, fieldName, tags); err != nil {
				return err
			}
		}
	}
	return nil
}

func sanitizeArrayOrSlice(field reflect.Value, name, parentTags string) error {
	for j := 0; j < field.Len(); j++ {
		v := field.Index(j)
		if v.Kind() != reflect.Pointer {
			v = v.Addr()
		}
		if err := sanitize(v, name, parentTags); err != nil {
			return fmt.Errorf("unable to sanitize slice/array element at index '%d': %w", j, err)
		}
	}
	return nil
}

func sanitizeMap(field reflect.Value, name, parentTags string) error {
	iter := field.MapRange()
	for iter.Next() {
		key, value := iter.Key(), iter.Value()
		needsCopy := value.Kind() != reflect.Pointer

		var tempValue reflect.Value
		if needsCopy {
			// map values are not addressable
			tempValue = reflect.New(value.Type()).Elem()
			tempValue.Set(value)
			value = tempValue.Addr()
		}
		if err := sanitize(value, name, parentTags); err != nil {
			return fmt.Errorf("unable to sanitize map value with key '%s': %w", key.String(), err)
		}
		if needsCopy {
			// Copy modified value back to map
			field.SetMapIndex(key, tempValue)
		}
	}
	return nil
}

func sanitizeValue(elem reflect.Value, name, tags string) error {
	kind := elem.Kind()
	for tag := range strings.SplitSeq(tags, ",") {
		if tag == "" {
			continue
		}
		tagKeyValue := strings.Split(tag, "=")
		switch tagKeyValue[0] {
		case "path_clean":
			if kind != reflect.String {
				return fmt.Errorf("sanitize tag '%s' on '%s' only works on strings", tagKeyValue[0], name)
			}
			path := elem.String()
			if len(path) == 0 {
				return nil
			}
			elem.SetString(filepath.Clean(path))
		case "path_abs":
			if kind != reflect.String {
				return fmt.Errorf("sanitize tag '%s' on '%s' only works on strings", tagKeyValue[0], name)
			}
			path := elem.String()
			if len(path) == 0 {
				return nil
			}
			apath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("failed to get absolute path for '%s': %w", path, err)
			}
			elem.SetString(apath)
		case "path_toslash":
			if kind != reflect.String {
				return fmt.Errorf("sanitize tag '%s' on '%s' only works on strings", tagKeyValue[0], name)
			}
			path := elem.String()
			if len(path) == 0 {
				return nil
			}
			elem.SetString(filepath.ToSlash(path))
		case "assure_dir_exists":
			if kind != reflect.String {
				return fmt.Errorf("sanitize tag '%s' on '%s' only works on strings", tagKeyValue[0], name)
			}
			dir := elem.String()
			if len(dir) == 0 {
				return nil
			}
			if err := os.MkdirAll(dir, 0777); err != nil {
				return fmt.Errorf("failed to create directory '%s': %w", dir, err)
			}
		case "assure_dir_exists_for_file":
			if kind != reflect.String {
				return fmt.Errorf("sanitize tag '%s' on '%s' only works on strings", tagKeyValue[0], name)
			}
			dir := filepath.Dir(elem.String())
			if len(dir) == 0 {
				return nil
			}
			if err := os.MkdirAll(dir, 0777); err != nil {
				return fmt.Errorf("failed to create directory '%s': %w", dir, err)
			}
		case "assure_file_access":
			if kind != reflect.String {
				return fmt.Errorf("sanitize tag '%s' on '%s' only works on strings", tagKeyValue[0], name)
			}
			if len(elem.String()) == 0 {
				return nil
			}
			fileName, err := filepath.Abs(elem.String())
			if err != nil {
				return fmt.Errorf("wrong file name '%s': %w", elem.String(), err)
			}
			if _, err := os.Stat(fileName); err != nil {
				return fmt.Errorf("file '%s' does not exists or is not accessible: %w", fileName, err)
			}
		case "oneof_or_tag":
			if kind != reflect.String {
				return fmt.Errorf("sanitize tag '%s' on '%s' only works on strings", tagKeyValue[0], name)
			}
			if len(elem.String()) == 0 {
				return nil
			}
			parts := strings.Split(tagKeyValue[1], " ")
			if len(parts) == 0 {
				return nil
			}
			if len(parts) == 1 {
				return fmt.Errorf("sanitize tag '%s' on '%s' must contain list of one_of tokens and another tag: %s", tagKeyValue[0], name, tagKeyValue[1])
			}

			tokens, op := parts[:len(parts)-1], parts[len(parts)-1]
			// check if value is in one_of tokens
			if slices.Contains(tokens, elem.String()) {
				return nil
			}
			// not found, apply operation
			return sanitizeValue(elem, name, op)

		// TODO: add more sanitize tags here when needed

		case "test_call":
			// should only be used in testing environment
			if !testing.Testing() {
				return nil
			}
			if kind != reflect.String {
				return fmt.Errorf("sanitize tag '%s' on '%s' only works on strings", tagKeyValue[0], name)
			}
			if len(tagKeyValue) < 2 {
				return fmt.Errorf("empty test_call on '%s', must be test_call=function", name)
			}
			testFunctions := sanitizeTestFunctions{}
			if err := testFunctions.invokeMethodByName(tagKeyValue[1], name, elem.String()); err != nil {
				return fmt.Errorf("failed to test_call: %w", err)
			}
		default:
			return fmt.Errorf("unknown sanitize tag: %s", tag)
		}
	}
	return nil
}
