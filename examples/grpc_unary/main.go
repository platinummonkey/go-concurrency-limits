package main

//go:generate protoc -I ./pb --go_out=plugins=grpc:${GOPATH}/src *.proto

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	golangGrpc "google.golang.org/grpc"

	"github.com/platinummonkey/go-concurrency-limits/examples/grpc_unary/pb"
	"github.com/platinummonkey/go-concurrency-limits/grpc"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

var options = struct {
	mode       string
	port       int
	numThreads int
}{
	mode:       "server",
	port:       8080,
	numThreads: 105,
}

func init() {
	flag.StringVar(&options.mode, "mode", options.mode, "choose `client` or `server` mode")
	flag.IntVar(&options.port, "port", options.port, "grpc port")
	flag.IntVar(&options.numThreads, "threads", options.numThreads, "number of client threads")
}

func checkOptions() {
	switch options.mode {
	case "client":
		fallthrough
	case "server":
		// no-op
	default:
		panic(fmt.Sprintf("invalid mode specified: '%s'", options.mode))
	}
}

type server struct {
}

func (s *server) PingPong(ctx context.Context, in *pb.Ping) (*pb.Pong, error) {
	log.Printf("Received: '%s'", in.GetMessage())
	// pretend to do some work
	time.Sleep(time.Millisecond * 10)
	return &pb.Pong{Message: in.GetMessage()}, nil
}

func main() {
	flag.Parse()
	checkOptions()
	switch options.mode {
	case "server":
		runServer()
	case "client":
		runClient()
	}
}

func runServer() {
	logger := limit.BuiltinLimitLogger{}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", options.port))
	if err != nil {
		panic(err)
	}

	serverLimit := limit.NewFixedLimit("server-fixed-limit", 10, nil)
	serverLimiter, err := limiter.NewDefaultLimiter(serverLimit, 1, 1000, 1e6, 100, strategy.NewSimpleStrategy(10), logger, nil)
	if err != nil {
		panic(err)
	}
	serverOpts := []grpc.InterceptorOption{grpc.WithName("grpc-unary-server"), grpc.WithLimiter(serverLimiter)}
	serverInterceptor := grpc.UnaryServerInterceptor(serverOpts...)
	svc := golangGrpc.NewServer(golangGrpc.UnaryInterceptor(serverInterceptor))
	s := &server{}
	pb.RegisterPingPongServer(svc, s)
	if err := svc.Serve(lis); err != nil {
		panic(nil)
	}
}

func runClient() {
	conn, err := golangGrpc.Dial(fmt.Sprintf("localhost:%d", options.port), golangGrpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := pb.NewPingPongClient(conn)

	wg := sync.WaitGroup{}
	wg.Add(options.numThreads)
	for i := 0; i < options.numThreads; i++ {
		go func(workerID int) {
			j := 0
			// do this as fast as possible
			for {
				queryServer(client, workerID, j)
				j++
			}
		}(i)
	}
	wg.Wait()
}

func queryServer(client pb.PingPongClient, workerID int, i int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	msg := &pb.Ping{
		Message: fmt.Sprintf("hello %d from %d", i, workerID),
	}
	r, err := client.PingPong(ctx, msg)
	if err != nil {
		log.Printf("[failed](%d - %d)\t - %v", workerID, i, err)
	} else {
		log.Printf("[pass](%d - %d)\t - %s", workerID, i, r.GetMessage())
	}
}
