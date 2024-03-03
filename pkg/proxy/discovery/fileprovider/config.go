package fileprovider

// Config defines a set of rules for the proxy to use.
type Config struct {
	Version    string   `yaml:"version"`
	NotMatched *Respond `yaml:"not-matched"`
	Rules      []Rule   `yaml:"rules"`
}

// Rule specifies a route matching rule.
type Rule struct {
	Match struct {
		URI string `yaml:"uri"`
	} `yaml:"match"`
	Respond Respond `yaml:"respond"`
}

// Respond specifies how the service should respond to the request.
type Respond struct {
	Body     *string `yaml:"body"`
	Metadata *struct {
		Header  map[string]string `yaml:"headers"`
		Trailer map[string]string `yaml:"trailer"`
	} `yaml:"metadata"`
	Status *struct {
		Code    string `yaml:"code"`
		Message string `yaml:"message"`
	} `yaml:"status"`
}
