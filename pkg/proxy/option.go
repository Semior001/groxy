package proxy

import "google.golang.org/grpc"

// Option is a functional option for the server.
type Option func(*Server)

// Version sets the version of the server.
func Version(v string) Option {
	return func(s *Server) { s.version = v }
}

// WithGRPCServerOptions sets the gRPC server options.
func WithGRPCServerOptions(opts ...grpc.ServerOption) Option {
	return func(o *Server) { o.serverOpts = append(o.serverOpts, opts...) }
}
