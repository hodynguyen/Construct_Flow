package bootstrap

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCClients struct {
	TaskServiceConn         *grpc.ClientConn
	UserServiceConn         *grpc.ClientConn
	NotificationServiceConn *grpc.ClientConn
}

func NewGRPCClients(cfg *Config) (*GRPCClients, error) {
	taskConn, err := grpc.NewClient(cfg.TaskServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dialing task-service at %s: %w", cfg.TaskServiceAddr, err)
	}

	userConn, err := grpc.NewClient(cfg.UserServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		taskConn.Close()
		return nil, fmt.Errorf("dialing user-service at %s: %w", cfg.UserServiceAddr, err)
	}

	notifConn, err := grpc.NewClient(cfg.NotificationServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		taskConn.Close()
		userConn.Close()
		return nil, fmt.Errorf("dialing notification-service at %s: %w", cfg.NotificationServiceAddr, err)
	}

	return &GRPCClients{
		TaskServiceConn:         taskConn,
		UserServiceConn:         userConn,
		NotificationServiceConn: notifConn,
	}, nil
}

func (c *GRPCClients) Close() {
	if c.TaskServiceConn != nil {
		c.TaskServiceConn.Close()
	}
	if c.UserServiceConn != nil {
		c.UserServiceConn.Close()
	}
	if c.NotificationServiceConn != nil {
		c.NotificationServiceConn.Close()
	}
}
