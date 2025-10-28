FROM docker.io/library/golang:1.24.0-alpine AS builder

# add local repo into the builder
ADD . /opt/build
WORKDIR /opt/build

# build the binary there
RUN CGO_ENABLED=0 go build -tags container -o fpl

# begin new container
FROM docker.io/library/alpine:3.22.2
WORKDIR /

# add some tools
RUN apk add --no-cache coreutils

# copy binary from builder to your desired location
COPY --from=builder /opt/build/fpl .
ENTRYPOINT ["/fpl", "-c", "config/config.yml"]
