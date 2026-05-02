# go-task/template

This is a forked version of Golang's standard
[`text/template`](https://pkg.go.dev/text/template) package. It is designed to
be a drop-in replacement for the original package with some additional features.

## Features

- `Template.Resolve` - This method will return a value from the given dataset by
  using Go's templating syntax. It supports any template consisting of a single
  `ActionNode`. This includes the use of dot notation and templating functions.
  This solves a limitation of the public API of the original package which meant
  that it was only ever possible to return a string representation of a value
  from a template.
