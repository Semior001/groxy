// Package _example embeds the example groxy configurations.
package _example

import "embed"

// ExampleConfigs contains all example configuration files.
//
//go:embed header-matching/config.yaml templating/config.yaml body-matching/config.yaml nested-messages/config.yaml error-responses/config.yaml upstream-forwarding/config.yaml uri-rewrite/config.yaml
var ExampleConfigs embed.FS
