package controller

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hodynguyen/construct-flow/apps/user-service/common"
	"github.com/hodynguyen/construct-flow/apps/user-service/entity/dto"
	"github.com/hodynguyen/construct-flow/apps/user-service/use-case/login"
	"github.com/hodynguyen/construct-flow/apps/user-service/use-case/register"
	userv1 "github.com/hodynguyen/construct-flow/gen/go/proto/user_service/v1"
)

// UserController implements the gRPC UserServiceServer.
type UserController struct {
	userv1.UnimplementedUserServiceServer
	registerUC *register.UseCase
	loginUC    *login.UseCase
}

func NewUserController(reg *register.UseCase, log *login.UseCase) *UserController {
	return &UserController{registerUC: reg, loginUC: log}
}

func (c *UserController) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterResponse, error) {
	resp, err := c.registerUC.Execute(ctx, dto.RegisterRequest{
		Email:       req.Email,
		Name:        req.Name,
		Password:    req.Password,
		Role:        req.Role,
		CompanyID:   req.CompanyId,
		CompanyName: req.CompanyName,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &userv1.RegisterResponse{
		User:      toProtoUser(resp.User),
		CompanyId: resp.CompanyID,
	}, nil
}

func (c *UserController) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginResponse, error) {
	resp, err := c.loginUC.Execute(ctx, dto.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &userv1.LoginResponse{
		AccessToken: resp.AccessToken,
		User:        toProtoUser(resp.User),
	}, nil
}

func (c *UserController) GetUser(_ context.Context, _ *userv1.GetUserRequest) (*userv1.UserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "GetUser not implemented")
}

// toProtoUser converts a dto.UserResponse to the protobuf User message.
func toProtoUser(u dto.UserResponse) *userv1.User {
	return &userv1.User{
		Id:        u.ID,
		CompanyId: u.CompanyID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      u.Role,
		CreatedAt: timestamppb.New(u.CreatedAt),
	}
}

// toGRPCError maps domain errors to gRPC status codes.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, common.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, common.ErrEmailExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, common.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, common.ErrNotFound), errors.Is(err, common.ErrCompanyNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
