package grpc

import (
	"context"

	golangGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/status"
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
			errResp, errCode, err := cfg.limitExceededResponseClassifier(ctx, info.FullMethod, req, cfg.limiter)
			return errResp, status.Error(errCode, err.Error())
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

func StreamServerInterceptor(opts ...InterceptorOption) golangGrpc.StreamServerInterceptor {
	cfg := new(interceptorConfig)
	defaults(cfg)
	for _, fn := range opts {
		fn(cfg)
	}
	return func(srv interface{}, ss golangGrpc.ServerStream, info *golangGrpc.StreamServerInfo, handler golangGrpc.StreamHandler) error {
		ctx := ss.Context()
		token, ok := cfg.limiter.Acquire(ctx)
		if !ok {
			_, errCode, err := cfg.limitExceededResponseClassifier(ctx, info.FullMethod, nil, cfg.limiter)
			return status.Error(errCode, err.Error())
		}
		err := handler(srv, ss)
		if err != nil {
			token.OnDropped()
			return err
		}
		token.OnSuccess()
		return nil
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
			_, errCode, err := cfg.limitExceededResponseClassifier(ctx, method, req, cfg.limiter)
			return status.Error(errCode, err.Error())
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
