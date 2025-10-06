FROM docker.io/library/golang:1.24.0-alpine AS builder
# add local repo into the builder
ADD . /opt/build
WORKDIR /opt/build

# build the binary there
RUN CGO_ENABLED=0 go build -tags container -o fpl

# use official prometheus container image
FROM quay.io/prometheus/prometheus AS prometheus

# use official grafana container image
FROM docker.io/grafana/grafana AS grafana

# begin new container
FROM docker.io/library/alpine

RUN apk add --no-cache supervisor

WORKDIR /flowpipeline-example

# copy binaries from builder to your desired location
COPY --from=builder /opt/build/fpl .

COPY --from=prometheus /bin/prometheus .
#COPY --from=prometheus /opt/build/prometheus/web/ui/static static

COPY --from=grafana /usr/share/grafana/ /usr/share/grafana/
COPY --from=grafana /etc/grafana/ /etc/grafana/

# add configuration files for all services
COPY examples/visualization/flowpipeline/flowpipeline-config.yml config/flowpipeline-config.yml

COPY examples/visualization/prometheus/prometheus-config.yml config/prometheus-config.yml

# add example grafana dashboards
COPY examples/visualization/grafana/grafana-datasource.yaml /usr/share/grafana/conf/provisioning/datasources/default.yml
COPY examples/visualization/grafana/grafana-dashboards.yaml /usr/share/grafana/conf/provisioning/dashboards/default.yml

COPY examples/visualization/grafana/dashboards/overview.json ./dashboards/overview.json
COPY examples/visualization/grafana/dashboards/raw-data.json ./dashboards/raw-data.json

# Create supervisord configuration
COPY examples/visualization/supervisord.conf /etc/supervisor/supervisord.conf

# Start grafana, prometheus & flowpipeline 
ENTRYPOINT ["supervisord", "-c", "/etc/supervisor/supervisord.conf"]
