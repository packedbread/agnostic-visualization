FROM envoyproxy/envoy:v1.18-latest
ENV ENVOY_UID=0 
RUN mkdir /var/log/envoy
COPY envoy.yaml /etc/envoy/envoy.yaml
