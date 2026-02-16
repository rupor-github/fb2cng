package gencfg

import (
	validator "github.com/go-playground/validator/v10"
)

type ValidatorOptions struct {
	custom validator.StructLevelFunc
}

// WithAdditionalChecks returns an option that registers a custom struct-level validation function.
func WithAdditionalChecks(fn validator.StructLevelFunc) func(*ValidatorOptions) {
	return func(opts *ValidatorOptions) {
		opts.custom = fn
	}
}

// Validate validates the data using the go-playground/validator package
func Validate(data any, options ...func(*ValidatorOptions)) error {

	opts := &ValidatorOptions{}
	for _, setOpt := range options {
		setOpt(opts)
	}

	// Create a new instance of the validator - we do not care about performance much, so we can create a new instance every time
	v := validator.New(validator.WithRequiredStructEnabled())
	if opts.custom != nil {
		v.RegisterStructValidation(opts.custom, data)
	}
	return v.Struct(data)
}
