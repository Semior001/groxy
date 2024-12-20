// Package middleware contains middlewares for gRPC unknown handlers.
//nolint:staticcheck // this file uses deprecated reflection API
package middleware

import (
	"github.com/Semior001/groxy/pkg/discovery"
	"google.golang.org/grpc"
	"strings"
	"fmt"
	rapi1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	rapi1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"errors"
	"io"
	"github.com/cappuccinotm/slogx"
	"log/slog"
	"context"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"github.com/samber/lo"
	"google.golang.org/grpc/status"
	"sort"
	"github.com/Semior001/groxy/pkg/grpcx"
)

// Reflector serves the reflection across multiple upstreams,
// by upstreaming the first request to each one and finding the
// one that doesn't respond a NotFound status and then piping
// the response back to the client.
type Reflector struct {
	Logger        *slog.Logger
	UpstreamsFunc func() []discovery.Upstream
}

// SRIClient is a shorthand for a server reflection stream client.
type SRIClient struct {
	Name string
	rapi1.ServerReflection_ServerReflectionInfoClient
}

// Middleware returns a middleware that reflects the request to the upstreams.
//goland:noinspection GoDeprecation
func (r Reflector) Middleware(next grpc.StreamHandler) grpc.StreamHandler {
	if r.Logger == nil {
		r.Logger = slog.Default()
	}

	return func(srv any, clientStream grpc.ServerStream) error {
		ctx := clientStream.Context()

		method, ok := grpc.Method(ctx)
		if !ok || !strings.HasPrefix(method, "/grpc.reflection.v1") {
			return next(srv, clientStream)
		}

		alpha := strings.HasPrefix(method, "/grpc.reflection.v1alpha")

		upstreams := r.UpstreamsFunc()
		for idx, up := range upstreams { // skip the reflection-disabled upstreams
			if !up.Reflection() {
				upstreams = append(upstreams[:idx], upstreams[idx+1:]...)
			}
		}

		clients := make([]SRIClient, len(upstreams))
		defer func() {
			for _, client := range clients {
				if client.ServerReflection_ServerReflectionInfoClient == nil {
					continue
				}

				if err := client.CloseSend(); err != nil {
					r.Logger.WarnContext(ctx, "failed to close the stream to upstream",
						slog.String("upstream", client.Name),
						slogx.Error(err))
					continue
				}

				r.Logger.DebugContext(ctx, "closed the stream to upstream",
					slog.String("upstream", client.Name))
			}
		}()
		for idx, upstream := range upstreams {
			cl, err := rapi1.NewServerReflectionClient(upstream).ServerReflectionInfo(ctx)
			if err != nil {
				r.Logger.WarnContext(ctx, "failed to make a new stream to upstream",
					slog.String("upstream", upstream.Name()),
					slog.String("target", upstream.Target()),
					slogx.Error(err))
				return status.Errorf(codes.Internal, "{groxy} can't make a new stream to upstream %s",
					upstream.Name())
			}

			clients[idx] = SRIClient{
				Name: upstream.Name(),
				ServerReflection_ServerReflectionInfoClient: cl,
			}

			r.Logger.DebugContext(ctx, "brought up a new stream to upstream",
				slog.String("upstream", upstream.Name()),
				slog.String("target", upstream.Target()))
		}

		for {
			recv := any(&rapi1.ServerReflectionRequest{})
			if alpha {
				recv = &rapi1alpha.ServerReflectionRequest{}
			}

			if err := clientStream.RecvMsg(recv); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}

				r.Logger.WarnContext(ctx, "failed to receive message", slogx.Error(err))
				return status.Error(codes.Internal, "{groxy} failed to receive message")
			}

			resp, err := r.reflect(ctx, r.asV1Request(recv), clients)
			if err != nil {
				if st := grpcx.StatusFromError(err); st != nil && grpcx.ClientCode(st.Code()) {
					return status.Errorf(st.Code(), "{groxy} received from one of upstreams: %s", st.Message())
				}
				r.Logger.WarnContext(ctx, "failed to reflect", slogx.Error(err))
				return status.Error(codes.Internal, "{groxy} failed to reflect")
			}

			result := any(resp)
			if alpha {
				result = r.asV1AlphaResponse(recv, resp)
			}

			if err = clientStream.SendMsg(result); err != nil {
				r.Logger.WarnContext(ctx, "failed to send message", slogx.Error(err))
				return status.Error(codes.Internal, "{groxy} failed to send message")
			}
		}
	}
}

func (r Reflector) reflect(
	ctx context.Context,
	req *rapi1.ServerReflectionRequest,
	ups []SRIClient,
) (*rapi1.ServerReflectionResponse, error) {
	resps := make([]*rapi1.ServerReflectionResponse, len(ups))
	ewg, ctx := errgroup.WithContext(ctx)
	for idx, up := range ups {
		idx, up := idx, up
		ewg.Go(func() error {
			if err := up.Send(req); err != nil {
				if errors.Is(err, io.EOF) {
					_, rerr := up.Recv()
					err = errors.Join(err, fmt.Errorf("recv after EOF: %w", rerr))
				}
				r.Logger.WarnContext(ctx, "failed to send reflection request",
					slog.String("upstream", up.Name),
					slogx.Error(err),
					slog.Any("trailers", up.Trailer()))
				return fmt.Errorf("send request to upstream: %w", err)
			}

			resp, err := up.Recv()
			if err != nil {
				r.Logger.WarnContext(ctx, "failed to send reflection request",
					slog.String("upstream", up.Name),
					slogx.Error(err))
				return fmt.Errorf("receive from upstream: %w", err)
			}

			if eresp := resp.GetErrorResponse(); eresp != nil && eresp.ErrorCode != int32(codes.NotFound) {
				r.Logger.WarnContext(ctx, "received error response from upstream",
					slog.String("upstream", up.Name),
					slog.String("error_message", eresp.ErrorMessage),
					slog.Int("error_code", int(eresp.ErrorCode)))
				return fmt.Errorf("error response from upstream: %s", eresp.ErrorMessage)
			}

			resps[idx] = resp
			return nil
		})
	}
	if err := ewg.Wait(); err != nil {
		return nil, fmt.Errorf("reflect: %w", err)
	}

	return r.mergeResponses(ctx, req, resps)
}

func (r Reflector) mergeResponses(
	ctx context.Context,
	req *rapi1.ServerReflectionRequest,
	resps []*rapi1.ServerReflectionResponse,
) (*rapi1.ServerReflectionResponse, error) {
	result := &rapi1.ServerReflectionResponse{OriginalRequest: req}
	switch req.MessageRequest.(type) {
	case *rapi1.ServerReflectionRequest_FileByFilename,
		*rapi1.ServerReflectionRequest_FileContainingExtension,
		*rapi1.ServerReflectionRequest_FileContainingSymbol:
		r.mergeDescriptorResponses(ctx, resps, result)
	case *rapi1.ServerReflectionRequest_ListServices:
		r.mergeServiceResponses(ctx, resps, result)
	case *rapi1.ServerReflectionRequest_AllExtensionNumbersOfType:
		r.respondAllExtensionNumbersOfType(resps, result)
	default:
		return nil, fmt.Errorf("unexpected message request: %T", req.MessageRequest)
	}

	if result.MessageResponse == nil {
		return &rapi1.ServerReflectionResponse{
			MessageResponse: &rapi1.ServerReflectionResponse_ErrorResponse{
				ErrorResponse: &rapi1.ErrorResponse{
					ErrorCode:    int32(codes.NotFound),
					ErrorMessage: "{groxy} didn't find any response among the upstreams",
				},
			},
		}, nil
	}

	return result, nil
}

func (r Reflector) mergeDescriptorResponses(
	ctx context.Context,
	resps []*rapi1.ServerReflectionResponse,
	resp *rapi1.ServerReflectionResponse,
) {
	result := &rapi1.ServerReflectionResponse_FileDescriptorResponse{
		FileDescriptorResponse: &rapi1.FileDescriptorResponse{},
	}

	for _, resp := range resps {
		if resp == nil {
			continue
		}

		if eresp := resp.GetErrorResponse(); eresp != nil {
			continue
		}

		fdresp := resp.GetFileDescriptorResponse()
		if fdresp == nil {
			r.Logger.WarnContext(ctx, "unexpected response type",
				slog.String("response_type", fmt.Sprintf("%T", resp.MessageResponse)))
			continue
		}

		result.FileDescriptorResponse.FileDescriptorProto = append(result.FileDescriptorResponse.FileDescriptorProto,
			fdresp.FileDescriptorProto...)
	}

	if len(result.FileDescriptorResponse.FileDescriptorProto) == 0 {
		return
	}

	resp.MessageResponse = result
}

func (r Reflector) mergeServiceResponses(
	ctx context.Context,
	resps []*rapi1.ServerReflectionResponse,
	result *rapi1.ServerReflectionResponse,
) {
	services := map[string]struct{}{}

	for _, resp := range resps {
		if eresp := resp.GetErrorResponse(); eresp != nil {
			continue
		}

		sresp := resp.GetListServicesResponse()
		if sresp == nil {
			r.Logger.WarnContext(ctx, "unexpected response type",
				slog.String("response_type", fmt.Sprintf("%T", resp.MessageResponse)))
			continue
		}

		for _, service := range sresp.Service {
			if _, ok := services[service.Name]; ok {
				r.Logger.WarnContext(ctx, "duplicate service reflected",
					slog.String("service", service.Name))
			}
			services[service.Name] = struct{}{}
		}
	}

	if len(services) == 0 {
		return
	}

	resp := &rapi1.ServerReflectionResponse_ListServicesResponse{
		ListServicesResponse: &rapi1.ListServiceResponse{
			Service: lo.Map(lo.Keys(services), func(service string, _ int) *rapi1.ServiceResponse {
				return &rapi1.ServiceResponse{Name: service}
			}),
		},
	}

	sort.Slice(resp.ListServicesResponse.Service, func(i, j int) bool {
		return resp.ListServicesResponse.Service[i].Name < resp.ListServicesResponse.Service[j].Name
	})

	result.MessageResponse = resp
}

// respondAllExtensionNumbersOfType returns the first non-error response.
func (r Reflector) respondAllExtensionNumbersOfType(
	resps []*rapi1.ServerReflectionResponse,
	result *rapi1.ServerReflectionResponse,
) {
	for _, resp := range resps {
		if eresp := resp.GetErrorResponse(); eresp != nil {
			continue
		}

		result.MessageResponse = resp.MessageResponse
		return
	}
}

//goland:noinspection GoDeprecation
func (r Reflector) asV1Request(recv any) *rapi1.ServerReflectionRequest {
	msg, ok := recv.(*rapi1alpha.ServerReflectionRequest)
	if !ok {
		return recv.(*rapi1.ServerReflectionRequest)
	}

	result := &rapi1.ServerReflectionRequest{Host: msg.Host}

	switch req := msg.MessageRequest.(type) {
	case *rapi1alpha.ServerReflectionRequest_FileByFilename:
		result.MessageRequest = &rapi1.ServerReflectionRequest_FileByFilename{
			FileByFilename: req.FileByFilename,
		}
	case *rapi1alpha.ServerReflectionRequest_FileContainingSymbol:
		result.MessageRequest = &rapi1.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: req.FileContainingSymbol,
		}
	case *rapi1alpha.ServerReflectionRequest_FileContainingExtension:
		result.MessageRequest = &rapi1.ServerReflectionRequest_FileContainingExtension{
			FileContainingExtension: &rapi1.ExtensionRequest{
				ContainingType:  req.FileContainingExtension.ContainingType,
				ExtensionNumber: req.FileContainingExtension.ExtensionNumber,
			},
		}
	case *rapi1alpha.ServerReflectionRequest_AllExtensionNumbersOfType:
		result.MessageRequest = &rapi1.ServerReflectionRequest_AllExtensionNumbersOfType{
			AllExtensionNumbersOfType: req.AllExtensionNumbersOfType,
		}
	case *rapi1alpha.ServerReflectionRequest_ListServices:
		result.MessageRequest = &rapi1.ServerReflectionRequest_ListServices{
			ListServices: req.ListServices,
		}
	default:
		panic(fmt.Sprintf("unexpected message request: %T", req))
	}

	return result
}

//goland:noinspection ALL
func (r Reflector) asV1AlphaResponse(req any, resp *rapi1.ServerReflectionResponse) any {
	result := &rapi1alpha.ServerReflectionResponse{OriginalRequest: req.(*rapi1alpha.ServerReflectionRequest)}

	switch resp := resp.MessageResponse.(type) {
	case *rapi1.ServerReflectionResponse_ErrorResponse:
		result.MessageResponse = &rapi1alpha.ServerReflectionResponse_ErrorResponse{
			ErrorResponse: &rapi1alpha.ErrorResponse{
				ErrorCode:    resp.ErrorResponse.ErrorCode,
				ErrorMessage: resp.ErrorResponse.ErrorMessage,
			},
		}
	case *rapi1.ServerReflectionResponse_FileDescriptorResponse:
		result.MessageResponse = &rapi1alpha.ServerReflectionResponse_FileDescriptorResponse{
			FileDescriptorResponse: &rapi1alpha.FileDescriptorResponse{
				FileDescriptorProto: resp.FileDescriptorResponse.FileDescriptorProto,
			},
		}
	case *rapi1.ServerReflectionResponse_ListServicesResponse:
		result.MessageResponse = &rapi1alpha.ServerReflectionResponse_ListServicesResponse{
			ListServicesResponse: &rapi1alpha.ListServiceResponse{
				Service: lo.Map(resp.ListServicesResponse.Service,
					func(svc *rapi1.ServiceResponse, _ int) *rapi1alpha.ServiceResponse {
						return &rapi1alpha.ServiceResponse{Name: svc.Name}
					}),
			},
		}
	default:
		panic(fmt.Sprintf("unexpected message response: %T", resp))
	}

	return result
}
