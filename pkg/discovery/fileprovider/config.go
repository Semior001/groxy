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
		URI    string            `yaml:"uri"`
		Header map[string]string `yaml:"header"`
		Body   *string           `yaml:"body,omitempty"`
	} `yaml:"match"`
	Respond Respond `yaml:"respond"`
}

// Values is an arbitrary structure.
type Values map[string]interface{}

// Respond specifies how the service should respond to the request.
type Respond struct {
	Stream *struct {
		Def    string   `yaml:"def"`
		Values []Values `yaml:"values"`
	} `yaml:"stream"`
	Body     *string `yaml:"body,omitempty"`
	Metadata *struct {
		Header  map[string]string `yaml:"header"`
		Trailer map[string]string `yaml:"trailer"`
	} `yaml:"metadata,omitempty"`
	Status *struct {
		Code    string `yaml:"code"`
		Message string `yaml:"message"`
	} `yaml:"status,omitempty"`
}
