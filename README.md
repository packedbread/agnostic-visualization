# Agnostic Visualization

Basically, it is a canvas drawing grpc api with persistent storage of scenes.

Current deployment is located here: nilserver.dynv6.net:6900

# Server general description

This includes 4 components (or services as per docker-compose config):

### 1. nginx_static:
Code is located under `server/frontend`. As it says in the name, it's the basic nginx static file server. Static files include grpc-web client.

### 2. grpc_proxy
Code (just a config file actually) located under `server/grpc_proxy`. Envoy proxy that forwards grpc-web requests to backend service. It is needed to translate http/1.1 traffic to http/2, which is used by grpc. For more info, look [here](https://blog.envoyproxy.io/envoy-and-grpc-web-a-fresh-new-alternative-to-rest-6504ce7eb880).

### 3. av_server
Code is located under `server/`. Main grpc service with all handlers. For persistence it stores drawings in mongodb.

### 4. mongo
No code here, just latest `mongodb` docker image. Everything is stored in database named `Visualization`. Scene info (mainly, authenticator) is stored in `scenes` collection. Drawings are stored in `drawings` collection (what a surprise).

## How to (re)build and run

1. Generate grpc and pb files from `.proto` using `generate.sh` (it's just a shorthand for 3 calls to `protoc`): 

```bash
$ ./generate.sh
```

2. Run `docker-compose` with build option:

```bash
$ docker-compose up --build
```

All build operations (frontend packing and main server binary building) are performed during multistage build of the respective images. See their `Dockerfile`s for more info on building.

## Deployment
If you want to deploy this for yourself, keep in mind that it requires 2 open (or forwarded) ports to work: 6900 (for static files) and 6901 (for grpc).

# Client
There is also a small client that can be used as console util for grpc api. It was used only for testing. 

