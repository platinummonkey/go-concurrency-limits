package grpc

import (
	"context"

	golangGrpc "google.golang.org/grpc"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

// StreamClientResponseClassifier is a method definition for defining custom response types to the limiter algorithm to
// correctly handle certain types of errors or embedded data.
type StreamClientResponseClassifier func(ctx context.Context, req interface{}, info *golangGrpc.StreamServerInfo, err error) ResponseType

// StreamServerResponseClassifier is a method definition for defining custom response types to the limiter algorithm to
// correctly handle certain types of errors or embedded data.
type StreamServerResponseClassifier func(
	ctx context.Context, req interface{}, info *golangGrpc.StreamServerInfo, err error,
) ResponseType


func defaultStreamClientResponseClassifier(
	ctx context.Context,
	req interface{},
	info *golangGrpc.StreamServerInfo,
	err error,
) ResponseType {
	if err != nil {
		return ResponseTypeDropped
	}
	return ResponseTypeSuccess
}

func defaultStreamServerResponseClassifier(
	ctx context.Context,
	req interface{},
	info *golangGrpc.StreamServerInfo,
	err error,
) ResponseType {
	if err != nil {
		return ResponseTypeDropped
	}
	return ResponseTypeSuccess
}

type streamInterceptorConfig struct {
	recvName                string
	sendName                string
	tags                    []string
	recvLimiter             core.Limiter
	sendLimiter             core.Limiter
	recvLimitExceededResponseClassifier LimitExceededResponseClassifier
	sendLimitExceededResponseClassifier LimitExceededResponseClassifier
	serverResponseClassifer StreamServerResponseClassifier
	clientResponseClassifer StreamClientResponseClassifier
}

// StreamInterceptorOption represents an option that can be passed to the stream
// client and server interceptors.
type StreamInterceptorOption func(*streamInterceptorConfig)

func streamDefaults(cfg *streamInterceptorConfig) {
	recvName := cfg.recvName
	if recvName == "" {
		recvName = "default-recv"
	}
	sendName := cfg.sendName
	if sendName == "" {
		sendName = "default-send"
	}
	tags := cfg.tags
	if tags == nil {
		tags = make([]string, 0)
	}
	cfg.recvLimiter, _ = limiter.NewDefaultLimiterWithDefaults(
		recvName,
		strategy.NewSimpleStrategy(1000),
		limit.NoopLimitLogger{},
		core.EmptyMetricRegistryInstance,
		tags...,
	)
	cfg.sendLimiter, _ = limiter.NewDefaultLimiterWithDefaults(
		sendName,
		strategy.NewSimpleStrategy(1000),
		limit.NoopLimitLogger{},
		core.EmptyMetricRegistryInstance,
		tags...,
	)
	cfg.recvLimitExceededResponseClassifier = defaultLimitExceededResponseClassifier
	cfg.sendLimitExceededResponseClassifier = defaultLimitExceededResponseClassifier
	cfg.clientResponseClassifer = defaultStreamClientResponseClassifier
	cfg.serverResponseClassifer = defaultStreamServerResponseClassifier
}

// WithStreamSendName sets the default SendMsg limiter name if the default limiter is used, otherwise unused.
func WithStreamSendName(name string) StreamInterceptorOption {
	return func(cfg *streamInterceptorConfig) {
		cfg.sendName = name
	}
}

// WithStreamRecvName sets the default RecvMsg limiter name if the default limiter is used, otherwise unused.
func WithStreamRecvName(name string) StreamInterceptorOption {
	return func(cfg *streamInterceptorConfig) {
		cfg.recvName = name
	}
}

// WithStreamTags sets the default limiter tags if the default limiter is used, otherwise unused.
func WithStreamTags(tags []string) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.tags = tags
	}
}

// WithStreamSendLimiter sets the given limiter for the intercepted stream client for SendMsg.
func WithStreamSendLimiter(limiter core.Limiter) StreamInterceptorOption {
	return func(cfg *streamInterceptorConfig) {
		cfg.sendLimiter = limiter
	}
}

// WithStreamRecvLimiter sets the given limiter for the intercepted stream client for RecvMsg.
func WithStreamRecvLimiter(limiter core.Limiter) StreamInterceptorOption {
	return func(cfg *streamInterceptorConfig) {
		cfg.recvLimiter = limiter
	}
}

// WithStreamSendLimitExceededResponseClassifier sets the response classifier for the intercepted stream client on SendMsg.
func WithStreamSendLimitExceededResponseClassifier(classifier LimitExceededResponseClassifier) StreamInterceptorOption {
	return func(cfg *streamInterceptorConfig) {
		cfg.sendLimitExceededResponseClassifier = classifier
	}
}

// WithStreamRecvLimitExceededResponseClassifier sets the response classifier for the intercepted stream client on RecvMsg.
func WithStreamRecvLimitExceededResponseClassifier(classifier LimitExceededResponseClassifier) StreamInterceptorOption {
	return func(cfg *streamInterceptorConfig) {
		cfg.recvLimitExceededResponseClassifier = classifier
	}
}

// WithStreamClientResponseTypeClassifier sets the response classifier for the intercepted client response
func WithStreamClientResponseTypeClassifier(classifier StreamClientResponseClassifier) StreamInterceptorOption {
	return func(cfg *streamInterceptorConfig) {
		cfg.clientResponseClassifer = classifier
	}
}

// WithStreamServerResponseTypeClassifier sets the response classifier for the intercepted server response
func WithStreamServerResponseTypeClassifier(classifier StreamServerResponseClassifier) StreamInterceptorOption {
	return func(cfg *streamInterceptorConfig) {
		cfg.serverResponseClassifer = classifier
	}
}

