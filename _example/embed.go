// Package _example embeds the example groxy configuration.
package _example

import _ "embed"

//go:embed mock.yaml
var Examples string
