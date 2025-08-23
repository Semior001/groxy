// Package _example provides the embedded example configuration file.
package _example

import _ "embed"

//go:embed mock.yaml
var Examples string
