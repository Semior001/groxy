// Package grpcx contains helper types and functions to work with
// gRPC streams and messages.
package grpcx

import (
	"context"
	"errors"
	"fmt"
	"io"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Direction describes the direction of the message.
type Direction uint8

const (
	// ClientToServer is a message from the client to the server.
	ClientToServer Direction = iota
	// ServerToClient is a message from the server to the client.
	ServerToClient
)

// Message describes a gRPC message.
type Message struct {
	Value     []byte
	Direction Direction

	// optional protobuf fully-qualified
	// name of the type of the message
	Descriptor string
}

// Pipe pipes the messages from the client stream to the server stream.
// Note that it closes the server stream when the client stream returned io.EOF.
func Pipe(server grpc.ClientStream, client grpc.ServerStream) error {
	ewg := &errgroup.Group{}
	ewg.Go(func() error {
		messageSent := false
		for {
			var msg []byte
			if err := server.RecvMsg(&msg); err != nil {
				// some servers (e.g. ones behind cloudflare) don't fully comply with gRPC spec and
				// can improperly close the stream without sending trailers, treat this as successful
				// completion if at least one message has been sent
				if isImproperCloseError(err) && messageSent {
					return nil
				}
				return fmt.Errorf("receive message from server stream: %w", err)
			}
			if err := client.SendMsg(msg); err != nil {
				return fmt.Errorf("send message to client stream: %w", err)
			}
			messageSent = true
		}
	})
	ewg.Go(func() error {
		for {
			var msg []byte
			if err := client.RecvMsg(&msg); err != nil {
				if errors.Is(err, io.EOF) {
					if err = server.CloseSend(); err != nil {
						return fmt.Errorf("close client stream: %w", err)
					}
					return nil
				}
				return fmt.Errorf("receive message from client stream: %w", err)
			}
			if err := server.SendMsg(msg); err != nil {
				return fmt.Errorf("send message to server stream: %w", err)
			}
		}
	})

	if err := ewg.Wait(); err != nil {
		return fmt.Errorf("pipe messages: %w", err)
	}

	return nil
}

// StatusFromError extracts the gRPC status from the error.
func StatusFromError(err error) *status.Status {
	var e interface {
		GRPCStatus() *status.Status
		error
	}
	if !errors.As(err, &e) {
		return nil
	}
	return e.GRPCStatus()
}

// ClientCode returns true if the code is a client-side error.
func ClientCode(code codes.Code) bool {
	switch code {
	case codes.Canceled,
		codes.Unknown,
		codes.DeadlineExceeded,
		codes.PermissionDenied,
		codes.ResourceExhausted,
		codes.Aborted,
		codes.Unimplemented,
		codes.Unavailable,
		codes.Unauthenticated:
		return true
	default:
		return false
	}
}

// StreamWithContext wraps the provided stream, replacing its context with the provided one.
func StreamWithContext(ctx context.Context, stream grpc.ServerStream) grpc.ServerStream {
	return contextedStream{ctx: ctx, ServerStream: stream}
}

type contextedStream struct {
	ctx context.Context
	grpc.ServerStream
}

func (s contextedStream) Context() context.Context { return s.ctx }

func isImproperCloseError(err error) bool {
	if err == nil {
		return false
	}
	st := StatusFromError(err)
	if st == nil {
		return false
	}
	return st.Code() == codes.Internal && st.Message() == "server closed the stream without sending trailers"
}
