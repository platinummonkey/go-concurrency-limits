package grpc

import (
	"context"
	"fmt"

	golangGrpc "google.golang.org/grpc"
)

// UnaryServerInterceptor will trace requests to the given grpc server.
func UnaryServerInterceptor(opts ...InterceptorOption) golangGrpc.UnaryServerInterceptor {
	cfg := new(interceptorConfig)
	defaults(cfg)
	for _, fn := range opts {
		fn(cfg)
	}
	return func(ctx context.Context, req interface{}, info *golangGrpc.UnaryServerInfo, handler golangGrpc.UnaryHandler) (interface{}, error) {
		token, ok := cfg.limiter.Acquire(ctx)
		if !ok {
			return nil, fmt.Errorf("limit exceeded for limiter=%v", cfg.limiter)
		}
		resp, err := handler(ctx, req)
		respType := cfg.serverResponseClassifer(ctx, req, info, resp, err)
		switch respType {
		case ResponseTypeSuccess:
			token.OnSuccess()
		case ResponseTypeIgnore:
			token.OnIgnore()
		case ResponseTypeDropped:
			token.OnDropped()
		}
		return resp, err
	}
}

// UnaryClientInterceptor will add tracing to a gprc client.
func UnaryClientInterceptor(opts ...InterceptorOption) golangGrpc.UnaryClientInterceptor {
	cfg := new(interceptorConfig)
	defaults(cfg)
	for _, fn := range opts {
		fn(cfg)
	}
	return func(ctx context.Context, method string, req, reply interface{}, cc *golangGrpc.ClientConn, invoker golangGrpc.UnaryInvoker, opts ...golangGrpc.CallOption) error {
		token, ok := cfg.limiter.Acquire(ctx)
		if !ok {
			return fmt.Errorf("limit exceeded for limiter=%v", cfg.limiter)
		}
		err := invoker(ctx, method, req, reply, cc, opts...)
		respType := cfg.clientResponseClassifer(ctx, method, req, reply, err)
		switch respType {
		case ResponseTypeSuccess:
			token.OnSuccess()
		case ResponseTypeIgnore:
			token.OnIgnore()
		case ResponseTypeDropped:
			token.OnDropped()
		}
		return err
	}
}
