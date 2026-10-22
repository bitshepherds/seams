package main

func serve(s *grpc.Server) {
	pb.RegisterPricingServer(s, &pricer{})
}
