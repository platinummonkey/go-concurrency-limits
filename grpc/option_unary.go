package grpc

import (
	"context"
	"fmt"

	golangGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

// ResponseType is the type of token release that should be specified to the limiter algorithm.
type ResponseType int

const (
	// ResponseTypeSuccess represents a successful response for the limiter algorithm
	ResponseTypeSuccess ResponseType = iota
	// ResponseTypeIgnore represents an ignorable error or response for the limiter algorithm
	ResponseTypeIgnore
	// ResponseTypeDropped represents a dropped request type for the limiter algorithm
	ResponseTypeDropped
)

// LimitExceededResponseClassifier is a method definition for defining the error response type when the limit is exceeded
// and a token is not able to be acquired. By default the RESOURCE_EXHUASTED type is returend.
type LimitExceededResponseClassifier func(ctx context.Context, method string, req interface{}, l core.Limiter) (interface{}, codes.Code, error)

// ClientResponseClassifier is a method definition for defining custom response types to the limiter algorithm to
// correctly handle certain types of errors or embedded data.
type ClientResponseClassifier func(ctx context.Context, method string, req, reply interface{}, err error) ResponseType

// ServerResponseClassifier is a method definition for defining custom response types to the limiter algorithm to
// correctly handle certain types of errors or embedded data.
type ServerResponseClassifier func(
	ctx context.Context, req interface{}, info *golangGrpc.UnaryServerInfo, resp interface{}, err error,
) ResponseType

func defaultLimitExceededResponseClassifier(
	ctx context.Context,
	method string,
	req interface{},
	l core.Limiter,
) (interface{}, codes.Code, error) {
	return nil, codes.ResourceExhausted, fmt.Errorf("limit exceeded for limiter=%v", l)
}

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
	name                            string
	tags                            []string
	limiter                         core.Limiter
	limitExceededResponseClassifier LimitExceededResponseClassifier
	serverResponseClassifer         ServerResponseClassifier
	clientResponseClassifer         ClientResponseClassifier
}

// InterceptorOption represents an option that can be passed to the grpc unary
// client and server interceptors.
type InterceptorOption func(*interceptorConfig)

func defaults(cfg *interceptorConfig) {
	name := cfg.name
	if name == "" {
		name = "default"
	}
	tags := cfg.tags
	if tags == nil {
		tags = make([]string, 0)
	}
	cfg.limiter, _ = limiter.NewDefaultLimiterWithDefaults(
		name,
		strategy.NewSimpleStrategy(1000),
		1000,
		limit.NoopLimitLogger{},
		core.EmptyMetricRegistryInstance,
		tags...,
	)
	cfg.limitExceededResponseClassifier = defaultLimitExceededResponseClassifier
	cfg.clientResponseClassifer = defaultClientResponseClassifier
	cfg.serverResponseClassifer = defaultServerResponseClassifier
}

// WithName sets the default limiter name if the default limiter is used, otherwise unused.
func WithName(name string) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.name = name
	}
}

// WithTags sets the default limiter tags if the default limiter is used, otherwise unused.
func WithTags(tags []string) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.tags = tags
	}
}

// WithLimiter sets the given limiter for the intercepted client.
func WithLimiter(limiter core.Limiter) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.limiter = limiter
	}
}

// WithLimitExceededResponseClassifier sets the response classifier for the intercepted client
func WithLimitExceededResponseClassifier(classifier LimitExceededResponseClassifier) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.limitExceededResponseClassifier = classifier
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
