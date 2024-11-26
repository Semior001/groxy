package grpcx

import (
	"google.golang.org/grpc"
	"context"
	"log/slog"
)

// ClientLogInterceptor logs the client stream messages.
func ClientLogInterceptor(logger *slog.Logger) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		logger.DebugContext(ctx, "invoking the client stream", slog.String("method", method))
		return streamer(ctx, desc, cc, method, opts...)
	}
}
