package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/Semior001/groxy/pkg/discovery/fileprovider"
	"github.com/invopop/jsonschema"
	"github.com/jessevdk/go-flags"
)

var opts struct {
	Output string `long:"output"      description:"Output file for the schema" default:"schema.json"`

	Title         string `long:"title"       description:"Title for the schema"`
	Description   string `long:"description" description:"Description for the schema"`
	SchemaVersion string `long:"schema-version"     description:"Version for the schema"`
	ID            string `long:"id"          description:"ID for the schema"`
}

func main() {
	if _, err := flags.Parse(&opts); err != nil {
		os.Exit(1)
	}

	reflector := &jsonschema.Reflector{FieldNameTag: "yaml"}

	schema := reflector.Reflect(&fileprovider.Config{})
	schema.Title = opts.Title
	schema.Description = opts.Description
	schema.Version = opts.SchemaVersion
	schema.Type = "object"

	bts, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal schema: %v", err)
	}

	if err = os.WriteFile(opts.Output, bts, 0o644); err != nil {
		log.Fatalf("failed to write schema to file: %v", err)
	}

	log.Printf("schema written to %s", opts.Output)
}
