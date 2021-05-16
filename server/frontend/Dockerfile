FROM node:12.18.2 as build

WORKDIR /root

COPY ./package.json /root/package.json
# COPY ./package-lock.json /root/package-lock.json

RUN yarn install
COPY *.js /root/
RUN yarn build

FROM nginx:latest

COPY --from=build /root/dist /usr/share/nginx/html
COPY ./index.html /usr/share/nginx/html
COPY ./nginx.conf /etc/nginx/conf.d/default.conf
