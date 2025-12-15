package gencfg

import (
	"fmt"
	"os"
	"regexp"

	yaml "gopkg.in/yaml.v3"
)

// ProcessingOptions holds options for expanding configuration files.
type ProcessingOptions struct {
	rootDir     string
	args        map[string]string
	doNotExpand map[string]bool
}

// WithRootDir sets root directory for template expansion.
// This is used to resolve relative paths - when empty current directory will be used.
func WithRootDir(rootDir string) func(*ProcessingOptions) {
	return func(opts *ProcessingOptions) {
		opts.rootDir = rootDir
	}
}

// WithArgument sets additional arguments for template expansion.
func WithArgument(name, value string) func(*ProcessingOptions) {
	return func(opts *ProcessingOptions) {
		if opts.args == nil {
			opts.args = make(map[string]string)
		}
		opts.args[name] = value
	}
}

// WithDoNotExpandField marks a field as not to be processed for template expansion.
func WithDoNotExpandField(name string) func(*ProcessingOptions) {
	return func(opts *ProcessingOptions) {
		if opts.doNotExpand == nil {
			opts.doNotExpand = make(map[string]bool)
		}
		opts.doNotExpand[name] = true
	}
}

type generationContext struct {
	opts *ProcessingOptions
	name string
}

// optimization - to avoid touching nodes which could not be templates.
var possiblyTemplate = regexp.MustCompile(`{{.*}}`)

func (gctx *generationContext) couldBeTemplate(field string) bool {
	return possiblyTemplate.MatchString(field)
}

// walk walks the YAML tree and expands fields if necessary
func (gctx *generationContext) walk(current, parent *yaml.Node, pos int) error {
	// iterate over all children of the current node before attempting to modify node itself
	// to avoid potential for loop - we have no idea how node will be expanded
	for i := 0; i < len(current.Content); i++ {
		if err := gctx.walk(current.Content[i], current, i%2); err != nil {
			return err
		}
	}
	// Value of any "terminal" node of a valid type could be "expanded" if necessary
	if parent != nil && parent.Kind == yaml.MappingNode {
		// first node in the mapping
		if pos == 0 {
			// save the name of the field, we may need it for expansion later
			gctx.name = current.Value
			return nil
		}
		// second node in the mapping - actual value, see if we could expand it
		if current.Tag == "!!str" && gctx.couldBeTemplate(current.Value) &&
			(gctx.opts.doNotExpand == nil || !gctx.opts.doNotExpand[gctx.name]) {

			value, err := expandField(gctx.name, current.Value, gctx.opts)
			if err != nil {
				return err
			}
			// Properly interpret expanded value - it may be YAML/JSON fragment
			var subnode yaml.Node
			if err := yaml.Unmarshal([]byte(value), &subnode); err != nil {
				return err
			}
			// Unwrap document node
			if subnode.Kind == yaml.DocumentNode {
				if len(subnode.Content) >= 1 {
					subnode = *subnode.Content[0]
				}
			}
			// Copy all fields from the expanded node to the current one - replacing node in place
			current.Alias = subnode.Alias
			current.Anchor = subnode.Anchor
			current.Content = subnode.Content
			current.Kind = subnode.Kind
			current.Tag = subnode.Tag
			if subnode.Style != 0 {
				current.Style = subnode.Style
			} else {
				if subnode.Tag == "!!bool" ||
					subnode.Tag == "!!null" ||
					subnode.Tag == "!!int" ||
					subnode.Tag == "!!float" {
					// to keep results consistent with our existing puppet implementation
					current.Style = yaml.FlowStyle
				}
				// TODO: see if style changes are needed for anything else "!!timestamp" "!!seq" "!!map" "!!binary" "!!merge"
			}
			current.Value = subnode.Value
		}
	}
	return nil
}

// Process generates configuration file from template using nodes names and values.
func Process(src []byte, options ...func(*ProcessingOptions)) ([]byte, error) {

	opts := &ProcessingOptions{}
	for _, setOpt := range options {
		setOpt(opts)
	}

	if len(opts.rootDir) == 0 {
		pwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("unable to get current working directory: %w", err)
		}
		opts.rootDir = pwd
	}

	var tree yaml.Node
	if err := yaml.Unmarshal(src, &tree); err != nil {
		return nil, err
	}

	if err := (&generationContext{opts: opts}).walk(&tree, nil, 0); err != nil {
		return nil, err
	}

	output, err := yaml.Marshal(&tree)
	if err != nil {
		return nil, err
	}
	return output, nil
}
