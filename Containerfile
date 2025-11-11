FROM docker.io/library/golang:1.24.10-alpine@sha256:12c199a889439928e36df7b4c5031c18bfdad0d33cdeae5dd35b2de369b5fbf5 AS builder

# add local repo into the builder
ADD . /opt/build
WORKDIR /opt/build

# build the binary there
RUN CGO_ENABLED=0 go build -tags container -o fpl

# begin new container
FROM docker.io/library/alpine:3.22.2@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412
WORKDIR /

# add some tools
RUN apk add --no-cache coreutils

# copy binary from builder to your desired location
COPY --from=builder /opt/build/fpl .
ENTRYPOINT ["/fpl", "-c", "config/config.yml"]
