package main

import (
	"log"
	"net"
	"net/http"

	pb "speedcamera/internal/gen/camera"
	"speedcamera/internal/repository"
	"speedcamera/internal/service"
	grpctransport "speedcamera/internal/transport/grpc"
	httptransport "speedcamera/internal/transport/http"
	"google.golang.org/grpc"
)

func main() {
	// 1. Инициализация БД
	repo, err := repository.NewSQLiteRepo("camera.db")
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}

	// 2. Инициализация бизнес-логики
	svc := service.NewService(repo)

	// 3. Запуск REST API
	httpHandler := httptransport.NewHandler(svc)
	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)

	go func() {
		log.Println("REST API server started on :8080")
		if err := http.ListenAndServe(":8080", mux); err != nil {
			log.Fatalf("REST server failed: %v", err)
		}
	}()

	// 4. Запуск gRPC API
	grpcServer := grpc.NewServer()
	pb.RegisterCameraServiceServer(grpcServer, grpctransport.NewServer(svc))

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen gRPC: %v", err)
	}

	log.Println("gRPC server started on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}