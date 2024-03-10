package middleware

import (
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/cappuccinotm/slogx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/proto"
)

// Log logs the gRPC requests.
func Log(debug bool) func(next grpc.StreamHandler) grpc.StreamHandler {
	return func(next grpc.StreamHandler) grpc.StreamHandler {
		return func(srv any, stream grpc.ServerStream) (err error) {
			ctx := stream.Context()
			ss := &statsStream{ServerStream: stream}

			start := time.Now()
			defer func() {
				elapsed := time.Since(start)
				mtd, ok := grpc.Method(ctx)
				if !ok {
					mtd = "unknown"
				}

				pi, ok := peer.FromContext(ctx)
				if !ok {
					pi = &peer.Peer{Addr: &net.IPAddr{IP: net.IPv4zero}}
				}

				attrs := []any{
					slog.String("uri", mtd),
					slog.String("remote", pi.Addr.String()),
					slog.Duration("elapsed", elapsed),
					slog.Int("recv_count", ss.recvCount),
					slog.Int64("recv_size", ss.recvSize),
					slog.Int("send_count", ss.sendCount),
					slog.Int64("send_size", ss.sendSize),
					slogx.Error(err),
				}

				if debug {
					reqHeader, _ := metadata.FromIncomingContext(ctx)
					attrs = append(attrs,
						slog.Any("request_header", filterMD(reqHeader)),
						slog.Any("response_header", filterMD(ss.header)),
						slog.Any("response_trailer", filterMD(ss.trailer)),
					)
				}

				slog.InfoContext(ctx, "request", attrs...)
			}()

			return next(srv, ss)
		}
	}
}

var hideHeaders = map[string]struct{}{
	"authorization": {},
	"cookie":        {},
	"set-cookie":    {},
}

func filterMD(md metadata.MD) metadata.MD {
	if md == nil {
		return nil
	}

	out := make(metadata.MD)
	for k, v := range md {
		if _, ok := hideHeaders[k]; ok {
			out[k] = []string{"***"}
			continue
		}
		out[k] = v
	}

	return out
}

type statsStream struct {
	lo      sync.RWMutex
	header  metadata.MD
	trailer metadata.MD

	recvCount int
	recvSize  int64

	sendCount int
	sendSize  int64

	grpc.ServerStream
}

func (s *statsStream) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}

	s.lo.Lock()
	defer s.lo.Unlock()

	s.recvCount++
	if msg, ok := m.(proto.Message); ok {
		s.recvSize += int64(proto.Size(msg))
	}

	return nil
}

func (s *statsStream) SendMsg(m interface{}) error {
	if err := s.ServerStream.SendMsg(m); err != nil {
		return err
	}

	s.lo.Lock()
	defer s.lo.Unlock()

	s.sendCount++
	if msg, ok := m.(proto.Message); ok {
		s.sendSize += int64(proto.Size(msg))
	}

	return nil
}

func (s *statsStream) SetHeader(md metadata.MD) error {
	s.lo.Lock()
	defer s.lo.Unlock()

	s.header = metadata.Join(s.header, md)
	return s.ServerStream.SetHeader(md)
}

func (s *statsStream) SendHeader(md metadata.MD) error {
	s.lo.Lock()
	defer s.lo.Unlock()

	s.header = md
	return s.ServerStream.SendHeader(md)
}

func (s *statsStream) SetTrailer(md metadata.MD) {
	s.lo.Lock()
	defer s.lo.Unlock()

	s.trailer = md
	s.ServerStream.SetTrailer(md)
}
