package main

import (
	"log"

	"github.com/hodynguyen/construct-flow/apps/user-service/api/grpc/controller"
	"github.com/hodynguyen/construct-flow/apps/user-service/bootstrap"
	sqlrepo "github.com/hodynguyen/construct-flow/apps/user-service/repository/sql"
	"github.com/hodynguyen/construct-flow/apps/user-service/service"
	"github.com/hodynguyen/construct-flow/apps/user-service/use-case/login"
	"github.com/hodynguyen/construct-flow/apps/user-service/use-case/register"
	userv1 "github.com/hodynguyen/construct-flow/gen/go/proto/user_service/v1"
)

func main() {
	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	db, err := bootstrap.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}

	tokenSvc, err := service.NewJWTService(cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath)
	if err != nil {
		log.Fatalf("initializing JWT service: %v", err)
	}

	userRepo := sqlrepo.NewUserRepository(db)
	companyRepo := sqlrepo.NewCompanyRepository(db)

	registerUC := register.New(userRepo, companyRepo)
	loginUC := login.New(userRepo, tokenSvc)

	grpcServer := bootstrap.NewGRPCServer()
	userv1.RegisterUserServiceServer(grpcServer, controller.NewUserController(registerUC, loginUC))

	log.Printf("user-service gRPC listening on :%d", cfg.GRPCPort)
	if err := bootstrap.StartGRPCServer(grpcServer, cfg.GRPCPort); err != nil {
		log.Fatalf("starting gRPC server: %v", err)
	}
}
