package grpc

import (
	golangGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type ssRecvWrapper struct {
	golangGrpc.ServerStream
	info *golangGrpc.StreamServerInfo
	cfg  *streamInterceptorConfig
}

// RecvMsg wrapps the underlying StreamServer RecvMsg with the limiter.
func (s *ssRecvWrapper) RecvMsg(m interface{}) error {
	ctx := s.Context()
	token, ok := s.cfg.recvLimiter.Acquire(ctx)
	if !ok {
		_, errCode, err := s.cfg.recvLimitExceededResponseClassifier(ctx, s.info.FullMethod, m, s.cfg.recvLimiter)
		return status.Error(errCode, err.Error())
	}
	err := s.ServerStream.RecvMsg(m)
	if err != nil {
		respType := s.cfg.serverResponseClassifer(ctx, m, s.info, err)
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
	return nil
}

// SendMsg wrapps the underlying StreamServer SendMsg with the limiter.
func (s *ssRecvWrapper) SendMsg(m interface{}) error {
	ctx := s.Context()
	token, ok := s.cfg.recvLimiter.Acquire(ctx)
	if !ok {
		_, errCode, err := s.cfg.sendLimitExceededResponseClassifier(ctx, s.info.FullMethod, m, s.cfg.recvLimiter)
		return status.Error(errCode, err.Error())
	}
	err := s.ServerStream.SendMsg(m)
	if err != nil {
		respType := s.cfg.clientResponseClassifer(ctx, m, s.info, err)
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
	return nil
}

// StreamServerInterceptor will add tracing to a gprc streaming client.
func StreamServerInterceptor(opts ...StreamInterceptorOption) golangGrpc.StreamServerInterceptor {
	cfg := new(streamInterceptorConfig)
	streamDefaults(cfg)
	for _, fn := range opts {
		fn(cfg)
	}
	return func(srv interface{}, ss golangGrpc.ServerStream, info *golangGrpc.StreamServerInfo, handler golangGrpc.StreamHandler) error {
		wrappedSs := &ssRecvWrapper{
			ServerStream: ss,
			info:         info,
			cfg:          cfg,
		}
		return handler(srv, wrappedSs)
	}
}
