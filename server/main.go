package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	grpc "google.golang.org/grpc"
)

var port = flag.Int("port", 10101, "The server port")

func EnsureNoError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	flag.Parse()

	initMongo()
	defer stopMongo()

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	fmt.Printf("Listening on port %d\n", *port)
	EnsureNoError(err)

	var opts []grpc.ServerOption

	grpcServer := grpc.NewServer(opts...)
	RegisterDrawerServer(grpcServer, newDrawerServer())
	grpcServer.Serve(listener)
}
