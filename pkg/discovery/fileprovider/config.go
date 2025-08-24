package fileprovider

// Config defines a set of rules for the proxy to use.
type Config struct {
	Version    string              `yaml:"version"     jsonschema:"title=Config Version,description=The version of the config schema."`
	NotMatched *Respond            `yaml:"not-matched" jsonschema:"title=Default Response,description=The default response to return when no rules match."`
	Rules      []Rule              `yaml:"rules"       jsonschema:"title=Rules,description=A list of rules to match incoming requests against."`
	Upstreams  map[string]Upstream `yaml:"upstreams"   jsonschema:"title=Upstreams,description=A map of upstream services that can be forwarded to."`
}

// Upstream specifies a service to forward requests to.
type Upstream struct {
	Addr            string `yaml:"address"          jsonschema:"title=Address,description=The address of the upstream service, in the format host:port."`
	TLS             bool   `yaml:"tls"              jsonschema:"title=TLS,description=Whether to use TLS when connecting to the upstream service."`
	ServeReflection bool   `yaml:"serve-reflection" jsonschema:"title=Serve Reflection,description=Whether to include the reflection from the upstream service."`
}

// Rule specifies a route matching rule.
type Rule struct {
	Match struct {
		URI    string            `yaml:"uri"    jsonschema:"title=URI,description=The URI to match against."`
		Header map[string]string `yaml:"header,omitempty" jsonschema:"title=Header,description=A map of headers to match against."`
		Body   *string           `yaml:"body,omitempty"   jsonschema:"title=Body,description=The body to match against."`
	} `yaml:"match" jsonschema:"title=Match,description=The criteria to match incoming requests against."`
	Respond *Respond `yaml:"respond,omitempty" jsonschema:"title=Respond,description=How to respond to the request if it matches. Mutually exclusive with 'forward'."`
	Forward *Forward `yaml:"forward,omitempty" jsonschema:"title=Forward,description=How to forward the request if it matches. Mutually exclusive with 'respond'."`
}

// Forward specifies how the service should forward the request.
type Forward struct {
	Upstream string            `yaml:"upstream" jsonschema:"title=Upstream,description=The name of the upstream service to forward the request to."`
	Header   map[string]string `yaml:"header,omitempty"   jsonschema:"title=Header,description=A map of headers to add to the request when forwarding."`
}

// Respond specifies how the service should respond to the request.
type Respond struct {
	Body     *string `yaml:"body,omitempty" jsonschema:"title=Body,description=The body to include in the response."`
	Metadata *struct {
		Header  map[string]string `yaml:"header"  jsonschema:"title=Header,description=A map of headers to include in the response."`
		Trailer map[string]string `yaml:"trailer" jsonschema:"title=Trailer,description=A map of trailers to include in the response."`
	} `yaml:"metadata,omitempty" jsonschema:"title=Metadata,description=Additional metadata to include in the response."`
	Status *struct {
		Code    string `yaml:"code" jsonschema:"title=Code,description=The gRPC status code to include in the response."`
		Message string `yaml:"message" jsonschema:"title=Message,description=The gRPC status message to include in the response."`
	} `yaml:"status,omitempty" jsonschema:"title=Status,description=The gRPC status to include in the response. Mutually exclusive with 'body'."`
}
