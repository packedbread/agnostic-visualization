FROM golang:1.16.4-buster as build
WORKDIR /root

COPY server/*.go /root/
COPY go.mod /root/
COPY go.sum /root/
RUN go build -o server

FROM ubuntu:20.10

COPY --from=build /root/server /root/server
CMD /root/server
