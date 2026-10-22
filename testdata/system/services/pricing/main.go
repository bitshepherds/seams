package main

import (
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
)

func main() {
	r := chi.NewRouter()
	r.Get("/quotes/{sku}", quote)
	go func() { _ = http.ListenAndServe(":8080", r) }()

	srv := grpc.NewServer()
	pb.RegisterPricingServer(srv, &pricer{})
	lis, _ := net.Listen("tcp", ":9090")
	_ = srv.Serve(lis)
}
