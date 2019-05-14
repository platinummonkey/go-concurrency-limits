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

	"github.com/platinummonkey/go-concurrency-limits/examples/grpc_streaming/pb"
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

func (s *server) PingPong(ss pb.PingPong_PingPongServer) error {
	ping, err := ss.Recv()
	if err != nil {
		log.Printf("Recv Error: %v", err)
		return nil
	}
	log.Printf("Received: '%s'", ping.GetMessage())
	// pretend to do some work
	time.Sleep(time.Millisecond * 10)
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
	serverLimiterSend, err := limiter.NewDefaultLimiter(serverLimitSend, 1000, 10000, 1e5, 1000, strategy.NewSimpleStrategy(1000), logger, nil)
	if err != nil {
		panic(err)
	}
	serverLimitRecv := limit.NewFixedLimit("server-fixed-limit-recv", 10, nil)
	serverLimiterRecv, err := limiter.NewDefaultLimiter(serverLimitRecv, 1000, 10000, 1e5, 1000, strategy.NewSimpleStrategy(10), logger, nil)
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

func resetConnection() (*golangGrpc.ClientConn, pb.PingPong_PingPongClient) {
	conn, err := golangGrpc.Dial(fmt.Sprintf("localhost:%d", options.port), golangGrpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	clientConn := pb.NewPingPongClient(conn)
	ctx := context.Background()
	client, err := clientConn.PingPong(ctx)
	if err != nil {
		panic(err)
	}
	return conn, client
}

func runClient() {
	wg := sync.WaitGroup{}
	wg.Add(options.numThreads)
	for i := 0; i < options.numThreads; i++ {
		go func(workerID int) {
			conn, client := resetConnection()
			j := 0
			for {
				// do this as fast as possible
				err := queryServer(client, workerID, j)
				if err != nil {
					client.CloseSend()
					conn.Close()
					conn, client = resetConnection()
				}
				j++
			}
		}(i)
	}
	wg.Wait()
}

func queryServer(client pb.PingPong_PingPongClient, workerID int, i int) error {
	msg := &pb.Ping{
		Message: fmt.Sprintf("hello %d from %d", i, workerID),
	}
	err := client.Send(msg)
	if err != nil {
		log.Printf("[failed](%d - %d)\t - %v", workerID, i, err)
		return err
	}
	pong, err := client.Recv()
	if err != nil {
		log.Printf("[failed](%d - %d)\t - %v", workerID, i, err)
	} else {
		log.Printf("[pass](%d - %d)\t - %s", workerID, i, pong.GetMessage())
	}
	return err
}
