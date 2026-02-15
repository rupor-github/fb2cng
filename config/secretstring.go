package config

// SecretStringValue must be exported - used in tests.
const SecretStringValue = "<secret>"

// SecretString is a type that should be used for fields that should not be visible in logs.
type SecretString string

// String implements fmt.Stringer so that %v and %s never print the raw value.
func (s SecretString) String() string {
	if len(s) == 0 {
		return ""
	}
	return SecretStringValue
}

// GoString implements fmt.GoStringer so that %#v never prints the raw value.
func (s SecretString) GoString() string {
	if len(s) == 0 {
		return "SecretString()"
	}
	return "SecretString(" + SecretStringValue + ")"
}

// MarshalJSON marshals SecretString to JSON making sure that actual value is not visible.
func (s SecretString) MarshalJSON() ([]byte, error) {
	if len(s) == 0 {
		return []byte("null"), nil
	}
	return []byte("\"" + SecretStringValue + "\""), nil
}

// MarshalYAML marshals SecretString to YAML making sure that actual value is not visible.
func (s SecretString) MarshalYAML() (any, error) {
	if len(s) == 0 {
		return nil, nil
	}
	return SecretStringValue, nil
}
