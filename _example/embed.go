// Package _example embeds the example groxy configurations.
package _example

import "embed"

// ExampleConfigs contains all example configuration files.
//
//go:embed */config.yaml
var ExampleConfigs embed.FS
