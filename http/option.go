package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

// ResponseType is the type of token release that should be signalled to the
// limiter algorithm after a request completes.
type ResponseType int

const (
	// ResponseTypeSuccess indicates a successful response. The measured RTT is
	// used as a sample for the adaptive limit algorithm.
	ResponseTypeSuccess ResponseType = iota
	// ResponseTypeIgnore indicates a response that should not affect the limit
	// (e.g. a client-side 4xx that is not caused by server overload).
	ResponseTypeIgnore
	// ResponseTypeDropped indicates a dropped or rejected request that should
	// trigger an aggressive limit reduction (e.g. 429, 5xx, timeout).
	ResponseTypeDropped
)

// LimitExceededHandler is called when a concurrency token cannot be acquired.
// It is responsible for writing an error response to w. The default
// implementation writes HTTP 503 Service Unavailable.
type LimitExceededHandler func(w http.ResponseWriter, r *http.Request, l core.Limiter)

// ServerResponseClassifier classifies a completed server response so the
// limiter can update its estimate appropriately.
type ServerResponseClassifier func(r *http.Request, w *ResponseWriter, err error) ResponseType

// ClientResponseClassifier classifies a completed client response so the
// limiter can update its estimate appropriately.
type ClientResponseClassifier func(resp *http.Response, err error) ResponseType

func defaultLimitExceededHandler(w http.ResponseWriter, r *http.Request, l core.Limiter) {
	http.Error(w,
		fmt.Sprintf("concurrency limit exceeded for limiter=%v", l),
		http.StatusServiceUnavailable,
	)
}

// defaultServerResponseClassifier maps HTTP status codes to ResponseType:
//   - 1xx / 2xx / 3xx → Success
//   - 4xx (except 429) → Ignore  (client errors unrelated to server load)
//   - 429 / 5xx        → Dropped (server is overloaded or rejecting)
func defaultServerResponseClassifier(r *http.Request, w *ResponseWriter, err error) ResponseType {
	if err != nil {
		return ResponseTypeDropped
	}
	return classifyStatusCode(w.StatusCode())
}

// defaultClientResponseClassifier applies the same status-code mapping used
// by the server classifier.
func defaultClientResponseClassifier(resp *http.Response, err error) ResponseType {
	if err != nil {
		return ResponseTypeDropped
	}
	if resp == nil {
		return ResponseTypeIgnore
	}
	return classifyStatusCode(resp.StatusCode)
}

func classifyStatusCode(code int) ResponseType {
	switch {
	case code == http.StatusTooManyRequests:
		return ResponseTypeDropped
	case code >= 500:
		return ResponseTypeDropped
	case code >= 400:
		return ResponseTypeIgnore
	default:
		return ResponseTypeSuccess
	}
}

type interceptorConfig struct {
	name                    string
	tags                    []string
	limiter                 core.Limiter
	limitExceededHandler    LimitExceededHandler
	serverResponseClassifier ServerResponseClassifier
	clientResponseClassifier ClientResponseClassifier
}

// InterceptorOption is a functional option for configuring server middleware
// and client round-tripper interceptors.
type InterceptorOption func(*interceptorConfig)

func applyDefaults(cfg *interceptorConfig) {
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
		limit.NoopLimitLogger{},
		core.EmptyMetricRegistryInstance,
		tags...,
	)
	cfg.limitExceededHandler = defaultLimitExceededHandler
	cfg.serverResponseClassifier = defaultServerResponseClassifier
	cfg.clientResponseClassifier = defaultClientResponseClassifier
}

// WithName sets the limiter name used when the default limiter is created.
// Has no effect when WithLimiter is also provided.
func WithName(name string) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.name = name
	}
}

// WithTags sets metric tags used when the default limiter is created.
// Has no effect when WithLimiter is also provided.
func WithTags(tags []string) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.tags = tags
	}
}

// WithLimiter sets a custom limiter, replacing the default one.
func WithLimiter(l core.Limiter) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.limiter = l
	}
}

// WithLimitExceededHandler sets a custom handler called when the concurrency
// limit is exceeded. It must write a complete HTTP response.
func WithLimitExceededHandler(h LimitExceededHandler) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.limitExceededHandler = h
	}
}

// WithServerResponseClassifier sets a custom response classifier for the
// server middleware.
func WithServerResponseClassifier(c ServerResponseClassifier) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.serverResponseClassifier = c
	}
}

// WithClientResponseClassifier sets a custom response classifier for the
// client round-tripper.
func WithClientResponseClassifier(c ClientResponseClassifier) InterceptorOption {
	return func(cfg *interceptorConfig) {
		cfg.clientResponseClassifier = c
	}
}

// ResponseWriter is an http.ResponseWriter wrapper that captures the status
// code written by the downstream handler.
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// newResponseWriter wraps w, defaulting the status code to 200 OK.
func newResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

// WriteHeader captures the status code and delegates to the underlying writer.
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// StatusCode returns the HTTP status code written by the handler.
// Defaults to 200 if WriteHeader was never called.
func (rw *ResponseWriter) StatusCode() int {
	return rw.statusCode
}

// Unwrap returns the underlying http.ResponseWriter, enabling compatibility
// with middleware that uses http.ResponseController.
func (rw *ResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// contextKey is an unexported type used for context values set by this package.
type contextKey struct{}

// withLimiter stores the limiter in the request context so that handlers can
// retrieve it if needed (e.g. for custom classifiers).
func withLimiter(ctx context.Context, l core.Limiter) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// LimiterFromContext retrieves the Limiter stored in the context by the server
// middleware, or returns nil if none is present.
func LimiterFromContext(ctx context.Context) core.Limiter {
	l, _ := ctx.Value(contextKey{}).(core.Limiter)
	return l
}
