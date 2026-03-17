package http

import (
	"fmt"
	"net/http"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

// NewServerMiddleware returns an HTTP middleware that enforces the configured
// concurrency limit. When a token cannot be acquired the configured
// LimitExceededHandler is called instead of forwarding to the next handler.
//
// The middleware stores the Limiter in the request context so that downstream
// handlers and custom classifiers can access it via LimiterFromContext.
//
// Example:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/api", myHandler)
//	http.ListenAndServe(":8080", NewServerMiddleware(WithName("api"))(mux))
func NewServerMiddleware(opts ...InterceptorOption) func(http.Handler) http.Handler {
	cfg := new(interceptorConfig)
	applyDefaults(cfg)
	for _, o := range opts {
		o(cfg)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := withLimiter(r.Context(), cfg.limiter)
			r = r.WithContext(ctx)

			token, ok := cfg.limiter.Acquire(r.Context())
			if !ok {
				cfg.limitExceededHandler(w, r, cfg.limiter)
				return
			}

			rw := newResponseWriter(w)
			next.ServeHTTP(rw, r)

			respType := cfg.serverResponseClassifier(r, rw, nil)
			switch respType {
			case ResponseTypeSuccess:
				token.OnSuccess()
			case ResponseTypeIgnore:
				token.OnIgnore()
			case ResponseTypeDropped:
				token.OnDropped()
			}
		})
	}
}

// NewClientRoundTripper wraps the default (or a provided) http.RoundTripper
// with adaptive concurrency limiting. A request is rejected before being sent
// if no token is available; otherwise the token is released after the response
// is received and classified.
//
// Example:
//
//	client := &http.Client{
//	    Transport: NewClientRoundTripper(WithName("my-client")),
//	}
func NewClientRoundTripper(opts ...InterceptorOption) http.RoundTripper {
	cfg := new(interceptorConfig)
	applyDefaults(cfg)
	for _, o := range opts {
		o(cfg)
	}
	return &clientRoundTripper{cfg: cfg, base: http.DefaultTransport}
}

// NewClientRoundTripperWithBase is like NewClientRoundTripper but uses base as
// the underlying transport. This is useful for testing or when chaining
// multiple round-trippers.
func NewClientRoundTripperWithBase(base http.RoundTripper, opts ...InterceptorOption) http.RoundTripper {
	cfg := new(interceptorConfig)
	applyDefaults(cfg)
	for _, o := range opts {
		o(cfg)
	}
	return &clientRoundTripper{cfg: cfg, base: base}
}

type clientRoundTripper struct {
	cfg  *interceptorConfig
	base http.RoundTripper
}

func (c *clientRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	token, ok := c.cfg.limiter.Acquire(r.Context())
	if !ok {
		return nil, &LimitExceededError{Limiter: c.cfg.limiter}
	}

	resp, err := c.base.RoundTrip(r)

	respType := c.cfg.clientResponseClassifier(resp, err)
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

// LimitExceededError is returned by the client round-tripper when no
// concurrency token is available.
type LimitExceededError struct {
	Limiter core.Limiter
}

func (e *LimitExceededError) Error() string {
	return fmt.Sprintf("concurrency limit exceeded for limiter=%v", e.Limiter)
}
