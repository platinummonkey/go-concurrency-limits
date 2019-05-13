package main

//go:generate protoc -I ./pb --go_out=plugins=grpc:${GOPATH}/src *.proto

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	golangGrpc "google.golang.org/grpc"

	"github.com/platinummonkey/go-concurrency-limits/examples/grpc_streaming/pb"
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

func (s *server) PingPong(ss pb.PingPong_PingPongServer) error {
	ping, err := ss.Recv()
	if err != nil {
		log.Printf("Recv Error: %v", err)
		return nil
	}
	log.Printf("Received: '%s'", ping.GetMessage())
	err = ss.Send(&pb.Pong{Message: ping.GetMessage()})
	if err != nil {
		log.Printf("Send Error: %v", err)
	}
	return nil
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

	serverLimitSend := limit.NewFixedLimit("server-fixed-limit-send", 1000, nil)
	serverLimiterSend, err := limiter.NewDefaultLimiter(serverLimitSend, 0, 10000, 0, 10, strategy.NewSimpleStrategy(1000), logger, nil)
	if err != nil {
		panic(err)
	}
	serverLimitRecv := limit.NewFixedLimit("server-fixed-limit-recv", 10, nil)
	serverLimiterRecv, err := limiter.NewDefaultLimiter(serverLimitRecv, 0, 10000, 0, 10, strategy.NewSimpleStrategy(10), logger, nil)
	if err != nil {
		panic(err)
	}
	serverOpts := []grpc.StreamInterceptorOption{
		grpc.WithStreamSendName("grpc-stream-server-send"),
		grpc.WithStreamRecvName("grpc-stream-server-recv"),
		grpc.WithStreamSendLimiter(serverLimiterSend), // outbound guard
		grpc.WithStreamRecvLimiter(serverLimiterRecv), // inbound guard
	}
	serverInterceptor := grpc.StreamServerInterceptor(serverOpts...)
	svc := golangGrpc.NewServer(golangGrpc.StreamInterceptor(serverInterceptor))
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
	clientConn := pb.NewPingPongClient(conn)
	ctx := context.Background()
	client, err := clientConn.PingPong(ctx)
	if err != nil {
		panic(err)
	}

	i := 0
	for {
		// do this as fast as possible
		queryServer(client, i)
		i++
	}
}

func queryServer(client pb.PingPong_PingPongClient, i int) {
	msg := &pb.Ping{
		Message: fmt.Sprintf("hello %d", i),
	}
	err := client.Send(msg)
	if err != nil {
		log.Printf("[failed]\t - %v", err)
		return
	}
	pong, err := client.Recv()
	if err != nil {
		log.Printf("[failed]\t - %v", err)
	} else {
		log.Printf("[pass]\t - %s", pong.GetMessage())
	}
}
