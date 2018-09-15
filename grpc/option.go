package grpc

import (
	"context"

	golangGrpc "google.golang.org/grpc"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

type ResponseType int

const (
	// ResponseTypeSuccess represents a successful response for the limiter algorithm
	ResponseTypeSuccess ResponseType = iota
	// ResponseTypeIgnore represents an ignorable error or response for the limiter algorithm
	ResponseTypeIgnore
	// ResponseTypeDropped represents a dropped request type for the limiter algorithm
	ResponseTypeDropped
)

type ClientResponseClassifier func(ctx context.Context, method string, req, reply interface{}, err error) ResponseType
type ServerResponseClassifier func(
	ctx context.Context, req interface{}, info *golangGrpc.UnaryServerInfo, resp interface{}, err error,
) ResponseType

func defaultClientResponseClassifier(
	ctx context.Context,
	method string,
	req interface{},
	reply interface{},
	err error,
) ResponseType {
	if err != nil {
		return ResponseTypeDropped
	}
	return ResponseTypeSuccess
}

func defaultServerResponseClassifier(
	ctx context.Context,
	req interface{},
	info *golangGrpc.UnaryServerInfo,
	resp interface{},
	err error,
) ResponseType {
	if err != nil {
		return ResponseTypeDropped
	}
	return ResponseTypeSuccess
}

type interceptorConfig struct {
	limiter                 core.Limiter
	serverResponseClassifer ServerResponseClassifier
	clientResponseClassifer ClientResponseClassifier
}

// InterceptorOption represents an option that can be passed to the grpc unary
// client and server interceptors.
type InterceptorOption func(*interceptorConfig)

func defaults(cfg *interceptorConfig) {
	cfg.limiter, _ = limiter.NewDefaultLimiterWithDefaults(
		strategy.NewSimpleStrategy(1000),
		limit.NoopLimitLogger{},
		core.EmptyMetricRegistryInstance,
	)
	cfg.clientResponseClassifer = defaultClientResponseClassifier
	cfg.serverResponseClassifer = defaultServerResponseClassifier
}

// WithLimiter sets the given limiter for the intercepted client.
func WithLimiter(limiter core.Limiter) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.limiter = limiter
	}
}

// WithClientResponseTypeClassifier sets the response classifier for the intercepted client
func WithClientResponseTypeClassifier(classifier ClientResponseClassifier) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.clientResponseClassifer = classifier
	}
}

// WithServerResponseTypeClassifier sets the response classifier for the intercepted client
func WithServerResponseTypeClassifier(classifier ServerResponseClassifier) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.serverResponseClassifer = classifier
	}
}
