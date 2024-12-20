package fileprovider

// Config defines a set of rules for the proxy to use.
type Config struct {
	Version    string              `yaml:"version"`
	NotMatched *Respond            `yaml:"not-matched"`
	Rules      []Rule              `yaml:"rules"`
	Upstreams  map[string]Upstream `yaml:"upstreams"`
}

// Upstream specifies a service to forward requests to.
type Upstream struct {
	Addr            string `yaml:"address"`
	TLS             bool   `yaml:"tls"`
	ServeReflection bool   `yaml:"serve-reflection"`
}

// Rule specifies a route matching rule.
type Rule struct {
	Match struct {
		URI    string            `yaml:"uri"`
		Header map[string]string `yaml:"header"`
		Body   *string           `yaml:"body"`
	} `yaml:"match"`
	Respond *Respond `yaml:"respond,omitempty"`
	Forward *Forward `yaml:"forward,omitempty"`
}

// Forward specifies how the service should forward the request.
type Forward struct {
	Upstream string            `yaml:"upstream"`
	Header   map[string]string `yaml:"header"`
}

// Respond specifies how the service should respond to the request.
type Respond struct {
	Body     *string `yaml:"body"`
	Metadata *struct {
		Header  map[string]string `yaml:"header"`
		Trailer map[string]string `yaml:"trailer"`
	} `yaml:"metadata"`
	Status *struct {
		Code    string `yaml:"code"`
		Message string `yaml:"message"`
	} `yaml:"status"`
}
