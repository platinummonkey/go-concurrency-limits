package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

// newFixedLimiter creates a DefaultLimiter backed by a FixedLimit so the
// concurrency cap is exactly maxConcurrency for the lifetime of the test.
func newFixedLimiter(name string, maxConcurrency int) core.Limiter {
	l, err := limiter.NewDefaultLimiter(
		limit.NewFixedLimit(name, maxConcurrency, nil),
		1e9, 1e9, 1e5, 10,
		strategy.NewSimpleStrategy(maxConcurrency),
		limit.NoopLimitLogger{},
		core.EmptyMetricRegistryInstance,
	)
	if err != nil {
		panic(fmt.Sprintf("newFixedLimiter: %v", err))
	}
	return l
}

// exhaustLimiter acquires n tokens from l and returns a release function that
// calls OnIgnore on all of them.
func exhaustLimiter(l core.Limiter, n int) func() {
	tokens := make([]core.Listener, 0, n)
	for range n {
		tok, ok := l.Acquire(context.TODO())
		if !ok {
			break
		}
		tokens = append(tokens, tok)
	}
	return func() {
		for _, t := range tokens {
			t.OnIgnore()
		}
	}
}

// ---- ResponseWriter tests ---------------------------------------------------

func TestResponseWriter_DefaultStatusCode(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec)
	assert.Equal(t, http.StatusOK, rw.StatusCode())
}

func TestResponseWriter_CapturesWrittenStatusCode(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec)
	rw.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, rw.StatusCode())
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestResponseWriter_Unwrap(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec)
	assert.Equal(t, rec, rw.Unwrap())
}

// ---- classifyStatusCode tests -----------------------------------------------

func TestClassifyStatusCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code int
		want ResponseType
	}{
		{http.StatusOK, ResponseTypeSuccess},
		{http.StatusCreated, ResponseTypeSuccess},
		{http.StatusMovedPermanently, ResponseTypeSuccess},
		{http.StatusBadRequest, ResponseTypeIgnore},
		{http.StatusUnauthorized, ResponseTypeIgnore},
		{http.StatusNotFound, ResponseTypeIgnore},
		{http.StatusTooManyRequests, ResponseTypeDropped},
		{http.StatusInternalServerError, ResponseTypeDropped},
		{http.StatusBadGateway, ResponseTypeDropped},
		{http.StatusServiceUnavailable, ResponseTypeDropped},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.code), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, classifyStatusCode(tc.code))
		})
	}
}

// ---- LimiterFromContext tests ------------------------------------------------

func TestLimiterFromContext_NilWhenAbsent(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Nil(t, LimiterFromContext(req.Context()))
}

func TestLimiterFromContext_ReturnsStoredLimiter(t *testing.T) {
	t.Parallel()
	l := newFixedLimiter("ctx-test", 10)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(withLimiter(req.Context(), l))
	assert.Equal(t, l, LimiterFromContext(req.Context()))
}

// ---- Option tests -----------------------------------------------------------

func TestWithName(t *testing.T) {
	t.Parallel()
	cfg := new(interceptorConfig)
	applyDefaults(cfg)
	WithName("my-server")(cfg)
	assert.NotNil(t, cfg.limiter)
}

func TestWithLimiter(t *testing.T) {
	t.Parallel()
	l := newFixedLimiter("custom", 5)
	cfg := new(interceptorConfig)
	applyDefaults(cfg)
	WithLimiter(l)(cfg)
	assert.Equal(t, l, cfg.limiter)
}

func TestWithLimitExceededHandler(t *testing.T) {
	t.Parallel()
	called := false
	h := LimitExceededHandler(func(w http.ResponseWriter, r *http.Request, l core.Limiter) {
		called = true
		w.WriteHeader(http.StatusTooManyRequests)
	})
	cfg := new(interceptorConfig)
	applyDefaults(cfg)
	WithLimitExceededHandler(h)(cfg)
	cfg.limitExceededHandler(httptest.NewRecorder(), nil, nil)
	assert.True(t, called)
}

func TestWithServerResponseClassifier(t *testing.T) {
	t.Parallel()
	called := false
	c := ServerResponseClassifier(func(r *http.Request, w *ResponseWriter, err error) ResponseType {
		called = true
		return ResponseTypeIgnore
	})
	cfg := new(interceptorConfig)
	applyDefaults(cfg)
	WithServerResponseClassifier(c)(cfg)
	cfg.serverResponseClassifier(nil, nil, nil)
	assert.True(t, called)
}

func TestWithClientResponseClassifier(t *testing.T) {
	t.Parallel()
	called := false
	c := ClientResponseClassifier(func(resp *http.Response, err error) ResponseType {
		called = true
		return ResponseTypeIgnore
	})
	cfg := new(interceptorConfig)
	applyDefaults(cfg)
	WithClientResponseClassifier(c)(cfg)
	cfg.clientResponseClassifier(nil, nil)
	assert.True(t, called)
}

// ---- Server middleware tests ------------------------------------------------

func TestServerMiddleware_PassesRequestToHandler(t *testing.T) {
	t.Parallel()
	l := newFixedLimiter("server-pass", 10)
	middleware := NewServerMiddleware(WithLimiter(l))

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestServerMiddleware_LimiterStoredInContext(t *testing.T) {
	t.Parallel()
	l := newFixedLimiter("server-ctx", 10)
	middleware := NewServerMiddleware(WithLimiter(l))

	var ctxLimiter core.Limiter
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxLimiter = LimiterFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, l, ctxLimiter)
}

func TestServerMiddleware_RejectsWhenLimitExceeded(t *testing.T) {
	t.Parallel()
	l := newFixedLimiter("server-reject", 1)
	release := exhaustLimiter(l, 1)
	defer release()

	middleware := NewServerMiddleware(WithLimiter(l))

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rec, req)

	assert.False(t, called, "handler should not be called when limit is exceeded")
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestServerMiddleware_CustomLimitExceededHandler(t *testing.T) {
	t.Parallel()
	l := newFixedLimiter("server-custom-exceeded", 1)
	release := exhaustLimiter(l, 1)
	defer release()

	middleware := NewServerMiddleware(
		WithLimiter(l),
		WithLimitExceededHandler(func(w http.ResponseWriter, r *http.Request, _ core.Limiter) {
			w.WriteHeader(http.StatusTooManyRequests)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestServerMiddleware_SuccessResponseReleasesToken(t *testing.T) {
	t.Parallel()
	l := newFixedLimiter("server-release", 1)
	middleware := NewServerMiddleware(WithLimiter(l))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		middleware(handler).ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "request %d should succeed", i)
	}
}

func TestServerMiddleware_5xxClassifiedAsDropped(t *testing.T) {
	t.Parallel()
	dropCount := 0
	dropClassifier := ServerResponseClassifier(func(r *http.Request, w *ResponseWriter, err error) ResponseType {
		rt := defaultServerResponseClassifier(r, w, err)
		if rt == ResponseTypeDropped {
			dropCount++
		}
		return rt
	})

	l := newFixedLimiter("server-5xx", 10)
	middleware := NewServerMiddleware(WithLimiter(l), WithServerResponseClassifier(dropClassifier))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, 1, dropCount)
}

func TestServerMiddleware_4xxClassifiedAsIgnore(t *testing.T) {
	t.Parallel()
	ignoreCount := 0
	ignoreClassifier := ServerResponseClassifier(func(r *http.Request, w *ResponseWriter, err error) ResponseType {
		rt := defaultServerResponseClassifier(r, w, err)
		if rt == ResponseTypeIgnore {
			ignoreCount++
		}
		return rt
	})

	l := newFixedLimiter("server-4xx", 10)
	middleware := NewServerMiddleware(WithLimiter(l), WithServerResponseClassifier(ignoreClassifier))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rec, req)

	assert.Equal(t, 1, ignoreCount)
}

// ---- Client round-tripper tests ---------------------------------------------

// fakeRoundTripper is a test http.RoundTripper that returns a canned response.
type fakeRoundTripper struct {
	resp *http.Response
	err  error
}

func (f *fakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f.resp, f.err
}

func fakeResponse(code int) *http.Response {
	return &http.Response{StatusCode: code, Body: http.NoBody}
}

func TestClientRoundTripper_SuccessfulRequest(t *testing.T) {
	t.Parallel()
	l := newFixedLimiter("client-ok", 10)
	rt := NewClientRoundTripperWithBase(&fakeRoundTripper{resp: fakeResponse(http.StatusOK)}, WithLimiter(l))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClientRoundTripper_RejectsWhenLimitExceeded(t *testing.T) {
	t.Parallel()
	l := newFixedLimiter("client-reject", 1)
	release := exhaustLimiter(l, 1)
	defer release()

	rt := NewClientRoundTripperWithBase(&fakeRoundTripper{resp: fakeResponse(http.StatusOK)}, WithLimiter(l))
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	resp, err := rt.RoundTrip(req)

	assert.Nil(t, resp)
	require.Error(t, err)
	var limitErr *LimitExceededError
	assert.True(t, errors.As(err, &limitErr), "expected LimitExceededError, got %T: %v", err, err)
}

func TestClientRoundTripper_ReleasesTokenAfterSuccess(t *testing.T) {
	t.Parallel()
	l := newFixedLimiter("client-release", 1)
	rt := NewClientRoundTripperWithBase(&fakeRoundTripper{resp: fakeResponse(http.StatusOK)}, WithLimiter(l))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err, "request %d should succeed", i)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}

func TestClientRoundTripper_TransportError(t *testing.T) {
	t.Parallel()
	transportErr := errors.New("connection refused")
	l := newFixedLimiter("client-transport-err", 10)
	rt := NewClientRoundTripperWithBase(&fakeRoundTripper{err: transportErr}, WithLimiter(l))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	resp, err := rt.RoundTrip(req)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, transportErr)
}

func TestClientRoundTripper_429ClassifiedAsDropped(t *testing.T) {
	t.Parallel()
	dropCount := 0
	dropClassifier := ClientResponseClassifier(func(resp *http.Response, err error) ResponseType {
		rt := defaultClientResponseClassifier(resp, err)
		if rt == ResponseTypeDropped {
			dropCount++
		}
		return rt
	})
	l := newFixedLimiter("client-429", 10)
	rt := NewClientRoundTripperWithBase(
		&fakeRoundTripper{resp: fakeResponse(http.StatusTooManyRequests)},
		WithLimiter(l),
		WithClientResponseClassifier(dropClassifier),
	)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	_, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 1, dropCount)
}

func TestClientRoundTripper_4xxClassifiedAsIgnore(t *testing.T) {
	t.Parallel()
	ignoreCount := 0
	ignoreClassifier := ClientResponseClassifier(func(resp *http.Response, err error) ResponseType {
		rt := defaultClientResponseClassifier(resp, err)
		if rt == ResponseTypeIgnore {
			ignoreCount++
		}
		return rt
	})
	l := newFixedLimiter("client-4xx", 10)
	rt := NewClientRoundTripperWithBase(
		&fakeRoundTripper{resp: fakeResponse(http.StatusNotFound)},
		WithLimiter(l),
		WithClientResponseClassifier(ignoreClassifier),
	)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	_, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, 1, ignoreCount)
}

func TestLimitExceededError_Message(t *testing.T) {
	t.Parallel()
	err := &LimitExceededError{Limiter: newFixedLimiter("err-msg", 1)}
	assert.Contains(t, err.Error(), "concurrency limit exceeded")
}
