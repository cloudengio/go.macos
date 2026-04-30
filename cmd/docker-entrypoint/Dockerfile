# Syntax=docker/dockerfile:1

# Stage 1: Build the binary
FROM golang:1.26 AS builder

WORKDIR /go/src/app

COPY . .
# Use a module name that doesn't exist to avoid confusion with the actual module
# we are testing here.
RUN go mod init cloudeng.io/macos/cmd/docker-entrypoint-test && \
    go mod download && \
    go mod tidy
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o docker-entrypoint .

# Stage 2: Create the final image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates
RUN apk add keyutils

COPY --from=builder /go/src/app/docker-entrypoint /usr/local/bin/docker-entrypoint

# Usage:
# docker build . -t ep-test 
# go run . run --keychain-item=<keys> -- --rm ep-test keyctl list @s
#

ENTRYPOINT ["docker-entrypoint", "entrypoint"]
