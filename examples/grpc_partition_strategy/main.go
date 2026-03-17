// Package main demonstrates how to use LookupPartitionStrategy and
// PredicatePartitionStrategy with a gRPC server to enforce per-method
// concurrency limits.
//
// Both strategies allow you to divide a total concurrency limit among named
// partitions, each with a percentage of the total. A partition may borrow
// unused capacity from other partitions up to the overall limit.
//
// Run as a server:
//
//	go run main.go -mode server
//
// Run as a client (in a separate terminal):
//
//	go run main.go -mode client
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	golangGrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/examples/grpc_unary/pb"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
	"github.com/platinummonkey/go-concurrency-limits/strategy/matchers"
)

var options = struct {
	mode            string
	port            int
	numThreads      int
	strategyType    string
}{
	mode:         "server",
	port:         8080,
	numThreads:   20,
	strategyType: "lookup",
}

func init() {
	flag.StringVar(&options.mode, "mode", options.mode, "choose `client` or `server` mode")
	flag.IntVar(&options.port, "port", options.port, "grpc port")
	flag.IntVar(&options.numThreads, "threads", options.numThreads, "number of client threads")
	flag.StringVar(&options.strategyType, "strategy", options.strategyType, "partition strategy: `lookup` or `predicate`")
}

// server implements the PingPong gRPC service.
type server struct{}

func (s *server) PingPong(ctx context.Context, in *pb.Ping) (*pb.Pong, error) {
	log.Printf("Received: '%s'", in.GetMessage())
	// Simulate work that takes different durations depending on message type.
	if in.GetMessage() == "slow" {
		time.Sleep(50 * time.Millisecond)
	} else {
		time.Sleep(5 * time.Millisecond)
	}
	return &pb.Pong{Message: in.GetMessage()}, nil
}

func main() {
	flag.Parse()
	switch options.mode {
	case "server":
		runServer()
	case "client":
		runClient()
	default:
		panic(fmt.Sprintf("invalid mode: '%s'", options.mode))
	}
}

// ---- LookupPartitionStrategy example ----------------------------------------
//
// LookupPartitionStrategy uses a lookup function to map a context to a named
// partition. This is well-suited for gRPC when you want to limit by method name:
// inject info.FullMethod into the context so the lookup function can read it.

// lookupServerInterceptor is a gRPC server interceptor that:
//  1. Injects the full method name into the context.
//  2. Acquires a token from a LookupPartitionStrategy-backed limiter.
func lookupServerInterceptor(l core.Limiter) golangGrpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *golangGrpc.UnaryServerInfo,
		handler golangGrpc.UnaryHandler,
	) (interface{}, error) {
		// Inject the method name so the LookupPartitionStrategy can route it.
		ctx = context.WithValue(ctx, matchers.LookupPartitionContextKey, info.FullMethod)

		token, ok := l.Acquire(ctx)
		if !ok {
			return nil, status.Errorf(codes.ResourceExhausted,
				"concurrency limit exceeded for method %s", info.FullMethod)
		}
		resp, err := handler(ctx, req)
		if err != nil {
			token.OnDropped()
		} else {
			token.OnSuccess()
		}
		return resp, err
	}
}

// newLookupLimiter builds a limiter backed by LookupPartitionStrategy.
//
// The strategy divides the total limit (10) into two named partitions:
//   - "/main.PingPong/PingPong" gets 70 % (7 slots)
//   - "default" gets 30 % (3 slots) and catches all other methods
//
// A partition may borrow unused capacity from the other partition up to the
// total limit, so bursts on one method won't starve the other.
func newLookupLimiter() core.Limiter {
	const totalLimit = 10

	partitions := map[string]*strategy.LookupPartition{
		// 70 % reserved for the main PingPong method.
		"/main.PingPong/PingPong": strategy.NewLookupPartitionWithMetricRegistry(
			"/main.PingPong/PingPong",
			0.7,
			1,
			core.EmptyMetricRegistryInstance,
		),
		// 30 % for everything else (acts as the default bucket).
		"default": strategy.NewLookupPartitionWithMetricRegistry(
			"default",
			0.3,
			1,
			core.EmptyMetricRegistryInstance,
		),
	}

	// The lookup function reads the method name we injected in the interceptor.
	// If the method has no dedicated partition it falls back to "default".
	lookupFn := func(ctx context.Context) string {
		if method, ok := ctx.Value(matchers.LookupPartitionContextKey).(string); ok {
			if _, exists := partitions[method]; exists {
				return method
			}
		}
		return "default"
	}

	strat, err := strategy.NewLookupPartitionStrategyWithMetricRegistry(
		partitions,
		lookupFn,
		totalLimit,
		core.EmptyMetricRegistryInstance,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create lookup partition strategy: %v", err))
	}

	l, err := limiter.NewDefaultLimiter(
		limit.NewFixedLimit("lookup-partition-limiter", totalLimit, nil),
		1e6, 1e6, 1e6, 100,
		strat,
		limit.BuiltinLimitLogger{},
		core.EmptyMetricRegistryInstance,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create limiter: %v", err))
	}
	return l
}

// ---- PredicatePartitionStrategy example --------------------------------------
//
// PredicatePartitionStrategy evaluates each partition's predicate function
// against the context in order and routes the request to the first match.
// This is useful when you want to classify requests by arbitrary context
// values (e.g., a "priority" header, user tier, or request type).

// predicateServerInterceptor is a gRPC server interceptor that:
//  1. Classifies the request as "fast" or "slow" based on message content.
//  2. Injects that classification into the context so the predicate matchers
//     can identify the correct partition.
func predicateServerInterceptor(l core.Limiter) golangGrpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *golangGrpc.UnaryServerInfo,
		handler golangGrpc.UnaryHandler,
	) (interface{}, error) {
		// Classify the request.  In a real server this could be a metadata
		// header, user role, or any other context-derivable value.
		reqType := "fast"
		if ping, ok := req.(*pb.Ping); ok && ping.GetMessage() == "slow" {
			reqType = "slow"
		}
		// Inject the classification so StringPredicateMatcher can read it.
		ctx = context.WithValue(ctx, matchers.StringPredicateContextKey, reqType)

		token, ok := l.Acquire(ctx)
		if !ok {
			return nil, status.Errorf(codes.ResourceExhausted,
				"concurrency limit exceeded for request type %s on %s", reqType, info.FullMethod)
		}
		resp, err := handler(ctx, req)
		if err != nil {
			token.OnDropped()
		} else {
			token.OnSuccess()
		}
		return resp, err
	}
}

// newPredicateLimiter builds a limiter backed by PredicatePartitionStrategy.
//
// The strategy has two partitions evaluated in order:
//   - "fast" matches when StringPredicateContextKey == "fast"  → 70 %
//   - "slow" matches when StringPredicateContextKey == "slow"  → 30 %
func newPredicateLimiter() core.Limiter {
	const totalLimit = 10

	partitions := []*strategy.PredicatePartition{
		strategy.NewPredicatePartitionWithMetricRegistry(
			"fast",
			0.7,
			matchers.StringPredicateMatcher("fast", false),
			core.EmptyMetricRegistryInstance,
		),
		strategy.NewPredicatePartitionWithMetricRegistry(
			"slow",
			0.3,
			matchers.StringPredicateMatcher("slow", false),
			core.EmptyMetricRegistryInstance,
		),
	}

	strat, err := strategy.NewPredicatePartitionStrategyWithMetricRegistry(
		partitions,
		totalLimit,
		core.EmptyMetricRegistryInstance,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create predicate partition strategy: %v", err))
	}

	l, err := limiter.NewDefaultLimiter(
		limit.NewFixedLimit("predicate-partition-limiter", totalLimit, nil),
		1e6, 1e6, 1e6, 100,
		strat,
		limit.BuiltinLimitLogger{},
		core.EmptyMetricRegistryInstance,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create limiter: %v", err))
	}
	return l
}

// ---- Server / client wiring --------------------------------------------------

func runServer() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", options.port))
	if err != nil {
		panic(err)
	}

	var interceptor golangGrpc.UnaryServerInterceptor
	switch options.strategyType {
	case "predicate":
		log.Println("Using PredicatePartitionStrategy")
		interceptor = predicateServerInterceptor(newPredicateLimiter())
	default:
		log.Println("Using LookupPartitionStrategy")
		interceptor = lookupServerInterceptor(newLookupLimiter())
	}

	svc := golangGrpc.NewServer(golangGrpc.UnaryInterceptor(interceptor))
	pb.RegisterPingPongServer(svc, &server{})
	log.Printf("Listening on :%d", options.port)
	if err := svc.Serve(lis); err != nil {
		panic(err)
	}
}

func runClient() {
	conn, err := golangGrpc.Dial(
		fmt.Sprintf("localhost:%d", options.port),
		golangGrpc.WithInsecure(), //nolint:staticcheck
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := pb.NewPingPongClient(conn)

	var wg sync.WaitGroup
	wg.Add(options.numThreads)
	for i := 0; i < options.numThreads; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				msg := "fast"
				if j%3 == 0 {
					msg = "slow"
				}
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				r, err := client.PingPong(ctx, &pb.Ping{Message: msg})
				cancel()
				if err != nil {
					log.Printf("[worker %d req %d] error: %v", id, j, err)
				} else {
					log.Printf("[worker %d req %d] ok: %s", id, j, r.GetMessage())
				}
			}
		}(i)
	}
	wg.Wait()
}
