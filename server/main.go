package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	grpc "google.golang.org/grpc"
)

var port = flag.Int("port", 10101, "The server port")
var mongoAddress = flag.String("mongo-address", "mongodb://mongo:27017", "Full mongo access url")

func EnsureNoError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	flag.Parse()

	initMongo(*mongoAddress)
	defer stopMongo()

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
	fmt.Printf("Listening on port %d\n", *port)
	EnsureNoError(err)

	var opts []grpc.ServerOption

	grpcServer := grpc.NewServer(opts...)
	RegisterDrawerServer(grpcServer, newDrawerServer())
	grpcServer.Serve(listener)
}
