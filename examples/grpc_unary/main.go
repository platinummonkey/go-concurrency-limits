package main

//go:generate protoc -I ./pb --go_out=plugins=grpc:${GOPATH}/src *.proto

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	golangGrpc "google.golang.org/grpc"

	"github.com/platinummonkey/go-concurrency-limits/examples/grpc_unary/pb"
	"github.com/platinummonkey/go-concurrency-limits/grpc"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

var options = struct {
	mode string
	port int
}{
	mode: "server",
	port: 8080,
}

func init() {
	flag.StringVar(&options.mode, "mode", options.mode, "choose `client` or `server` mode")
	flag.IntVar(&options.port, "port", options.port, "grpc port")
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
	serverLimiter, err := limiter.NewDefaultLimiter(serverLimit, 0, 10000, 0, 10, strategy.NewSimpleStrategy(10), logger, nil)
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

	i := 0
	for {
		// do this as fast as possible
		queryServer(client, i)
		i++
	}
}

func queryServer(client pb.PingPongClient, i int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	msg := &pb.Ping{
		Message: fmt.Sprintf("hello %d", i),
	}
	r, err := client.PingPong(ctx, msg)
	if err != nil {
		log.Printf("[failed]\t - %v", err)
	} else {
		log.Printf("[pass]\t - %s", r.GetMessage())
	}
}
