// Package grpcx contains helper types and functions to work with
// gRPC streams and messages.
package grpcx

import (
	"google.golang.org/grpc"
	"fmt"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/status"
	"errors"
	"google.golang.org/grpc/codes"
	"io"
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
func Pipe(server grpc.ClientStream, client grpc.ServerStream, opts ...PipeOption) error {
	o := pipeOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	ewg := &errgroup.Group{}
	ewg.Go(func() error {
		for {
			var msg []byte
			if err := server.RecvMsg(&msg); err != nil {
				return fmt.Errorf("receive message from client stream: %w", err)
			}
			for _, f := range o.beforeClientSend {
				f(server, client)
			}
			if err := client.SendMsg(msg); err != nil {
				return fmt.Errorf("send message to server stream: %w", err)
			}
		}
	})
	ewg.Go(func() error {
		for {
			var msg []byte
			if err := client.RecvMsg(&msg); err != nil {
				if errors.Is(err, io.EOF) {
					if err = server.CloseSend(); err != nil {
						return fmt.Errorf("close server stream: %w", err)
					}
					return nil
				}
				return fmt.Errorf("receive message from server stream: %w", err)
			}
			if err := server.SendMsg(msg); err != nil {
				return fmt.Errorf("send message to client stream: %w", err)
			}
		}
	})

	if err := ewg.Wait(); err != nil {
		return fmt.Errorf("pipe messages: %w", err)
	}

	return nil
}

// PipeOption is a pipe option.
type PipeOption func(*pipeOptions)

type pipeOptions struct {
	beforeClientSend []func(server grpc.ClientStream, client grpc.ServerStream)
}

// BeforeClientSend sets the function that is called before pipe sends the received from
// the server message to the client.
func BeforeClientSend(f func(server grpc.ClientStream, client grpc.ServerStream)) PipeOption {
	return func(o *pipeOptions) {
		o.beforeClientSend = append(o.beforeClientSend, f)
	}
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
