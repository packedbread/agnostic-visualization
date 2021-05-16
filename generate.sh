#!/usr/bin/env bash

for out in "client" "server"; do
    protoc --go_out=$out --go_opt=paths=source_relative --go-grpc_out=$out --go-grpc_opt=paths=source_relative service.proto
done

protoc --js_out=import_style=commonjs:server/frontend --grpc-web_out=import_style=commonjs,mode=grpcwebtext:server/frontend service.proto
