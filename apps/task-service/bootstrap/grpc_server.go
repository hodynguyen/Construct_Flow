package bootstrap

import (
	"fmt"
	"net"

	"go.elastic.co/apm/module/apmgrpc/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewGRPCServer() *grpc.Server {
	server := grpc.NewServer(
		grpc.UnaryInterceptor(apmgrpc.NewUnaryServerInterceptor()),
	)
	reflection.Register(server) // allows grpcurl introspection
	return server
}

func StartGRPCServer(server *grpc.Server, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("listening on port %d: %w", port, err)
	}
	return server.Serve(lis)
}
