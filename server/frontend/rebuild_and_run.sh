#!/usr/bin/env bash
docker container stop proxy
docker rmi proxy
docker build --tag proxy .
docker run -it --rm -d -v proxy_logs:/var/log/ -p 8080:80 --name proxy proxy
