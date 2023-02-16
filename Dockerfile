FROM golang:1.19-alpine as builder

RUN apk add --no-cache \
    make \
    git \
    bash \
    curl \
    gcc \
    g++

RUN mkdir -p /go/src/github.com/tikv/pd
WORKDIR /go/src/github.com/tikv/pd

# Cache dependencies
COPY go.mod .
COPY go.sum .

RUN GO111MODULE=on go mod download

COPY . .

RUN GOOS=linux go build -o ./autoscale/autoscale  tools/tiflash-autoscale/main.go 

# FROM alpine:3.5

# COPY --from=builder /go/src/github.com/tikv/pd/autoscale/k8splayground /autoscale
RUN cp /go/src/github.com/tikv/pd/autoscale/autoscale /autoscale

EXPOSE 1-65535

ENTRYPOINT ["/bin/sh", "-c", "/autoscale > /var/log/autoscale_std.log"]
