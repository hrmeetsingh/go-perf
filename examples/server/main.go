package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	bench "github.com/hrmeetsingh/go-perf/examples/gen/bench"
	examples "github.com/hrmeetsingh/go-perf/examples"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultPort = "50051"

// benchServer implements bench.BenchServiceServer with simulated performance behaviour.
type benchServer struct {
	bench.UnimplementedBenchServiceServer
}

// FastEcho returns the input message immediately.
// Base latency: ~2ms. Spike: 1-in-10 requests inflated to 20ms.
func (s *benchServer) FastEcho(ctx context.Context, req *bench.EchoRequest) (*bench.EchoResponse, error) {
	start := time.Now()
	examples.Sleep(2*time.Millisecond, req.UserTier, 0.10, 10.0)
	return &bench.EchoResponse{
		Message:   "echo: " + req.Message,
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

// ProcessOrder simulates order processing. Latency scales with payload size
// and user tier. 1-in-10 requests get a 10× spike.
func (s *benchServer) ProcessOrder(ctx context.Context, req *bench.OrderRequest) (*bench.OrderResponse, error) {
	base := examples.PayloadLatency(len(req.Payload))
	tiered := time.Duration(float64(base) * examples.TierMultiplier(req.UserTier))
	examples.Sleep(tiered, req.UserTier, 0.10, 10.0)

	st := "processed"
	if req.Amount <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "amount must be positive")
	}

	return &bench.OrderResponse{
		OrderId:     req.OrderId,
		Status:      st,
		ProcessedAt: time.Now().UnixMilli(),
	}, nil
}

// StreamEvents streams count event messages back to the client.
// Each message has ~10ms latency; 1-in-5 messages are delayed to 50ms.
func (s *benchServer) StreamEvents(req *bench.StreamRequest, stream bench.BenchService_StreamEventsServer) error {
	count := int(req.Count)
	if count <= 0 {
		count = 5
	}
	for i := 0; i < count; i++ {
		examples.Sleep(10*time.Millisecond, req.UserTier, 0.20, 5.0)

		msg := &bench.EventMessage{
			EventId:   fmt.Sprintf("%s-%04d", req.Topic, i),
			Topic:     req.Topic,
			Data:      fmt.Sprintf(`{"seq":%d,"topic":"%s"}`, i, req.Topic),
			Timestamp: time.Now().UnixMilli(),
		}
		if err := stream.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

// BidirectionalChat echoes each received ChatMessage back after a delay.
// Delay varies by user tier; 1-in-8 messages get a 10× spike.
func (s *benchServer) BidirectionalChat(stream bench.BenchService_BidirectionalChatServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		examples.Sleep(3*time.Millisecond, msg.UserTier, 0.125, 10.0)

		reply := &bench.ChatMessage{
			Sender:    "server",
			Text:      "pong: " + msg.Text,
			UserTier:  msg.UserTier,
			Timestamp: time.Now().UnixMilli(),
		}
		if err := stream.Send(reply); err != nil {
			return err
		}
	}
}

func main() {
	port := os.Getenv("BENCH_PORT")
	if port == "" {
		port = defaultPort
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	bench.RegisterBenchServiceServer(s, &benchServer{})

	log.Printf("BenchService listening on :%s", port)
	log.Printf("Methods: FastEcho, ProcessOrder, StreamEvents, BidirectionalChat")
	log.Printf("Performance patterns: payload-driven latency, tier multipliers, random spikes")

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
