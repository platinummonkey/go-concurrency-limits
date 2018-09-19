package matchers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringPredicateMatcher(t *testing.T) {
	asrt := assert.New(t)
	matcher := StringPredicateMatcher("foo", false)
	ctx1 := context.WithValue(context.Background(), StringPredicateContextKey, "foo")
	ctx2 := context.WithValue(context.Background(), StringPredicateContextKey, "Foo")
	ctx3 := context.WithValue(context.Background(), StringPredicateContextKey, "bar")

	// stringer test
	asrt.Equal("go-concurrency-limits|strategy|stringPredicate", StringPredicateContextKey.String())

	asrt.True(matcher(ctx1), "expected case match")
	asrt.False(matcher(ctx2), "expected case sensitive failure here")
	asrt.False(matcher(ctx3), "this shouldn't match")
	asrt.False(matcher(context.Background()), "expect default false")

	matcher = StringPredicateMatcher("foo", true)
	asrt.True(matcher(ctx1), "expected case match")
	asrt.True(matcher(ctx2), "expected case insensitive match")
	asrt.False(matcher(ctx3), "this shouldn't match")
}

func TestDefaultStringLookupFunc(t *testing.T) {
	asrt := assert.New(t)
	f := DefaultStringLookupFunc
	ctx1 := context.WithValue(context.Background(), LookupPartitionContextKey, "foo")
	ctx2 := context.WithValue(context.Background(), LookupPartitionContextKey, "bar")

	// stringer test
	asrt.Equal("go-concurrency-limits|strategy|stringLookup", LookupPartitionContextKey.String())

	asrt.Equal("foo", f(ctx1))
	asrt.Equal("bar", f(ctx2))
	asrt.Equal("", f(context.Background()), "expected default value")
}
