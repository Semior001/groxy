// Package grpcx contains helper types and functions to work with
// gRPC streams and messages.
package grpcx

import (
	"google.golang.org/grpc"
	"fmt"
	"golang.org/x/sync/errgroup"
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

// Pipe pipes the messages from the client stream to the server stream
// Note that it doesn't close either of streams in case of any error, this
// is always the consumer's responsibility.
func Pipe(server grpc.ClientStream, client grpc.ServerStream, msgs ...Message) error {
	for idx, msg := range msgs {
		switch msg.Direction {
		case ClientToServer:
			if err := server.SendMsg(msg.Value); err != nil {
				return fmt.Errorf("send %d first message to server stream: %w", idx, err)
			}
		case ServerToClient:
			if err := client.SendMsg(msg.Value); err != nil {
				return fmt.Errorf("send %d first message to client stream: %w", idx, err)
			}
		}
	}

	ewg := &errgroup.Group{}
	ewg.Go(func() error {
		for {
			var msg []byte
			if err := server.RecvMsg(&msg); err != nil {
				return fmt.Errorf("receive message from client stream: %w", err)
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

// StatusFromError extracts the gRPC status from the error.
func StatusFromError(err error) *status.Status {
	for {
		if se, ok := err.(interface{ GRPCStatus() *status.Status }); ok {
			return se.GRPCStatus()
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
			continue
		}
		break
	}
	return nil
}
