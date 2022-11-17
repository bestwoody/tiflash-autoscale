FROM golang:1.18-alpine as builder

RUN apk add --no-cache \
    make \
    git \
    bash \
    curl \
    gcc \
    g++

# Install jq for pd-ctl
# RUN cd / && \
#     wget https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 -O jq && \
#     chmod +x jq

RUN mkdir -p /go/src/github.com/tikv/pd
WORKDIR /go/src/github.com/tikv/pd

# Cache dependencies
COPY go.mod .
COPY go.sum .

RUN GO111MODULE=on go mod download

COPY . .

# RUN make
RUN GOOS=linux go build -o ./autoscale  tools/tiflash-autoscale/k8splayground.go 

FROM alpine:3.5

COPY --from=builder /go/src/github.com/tikv/pd/autoscale/k8splayground /autoscale

EXPOSE 1-65535

ENTRYPOINT ["/bin/sh", "-c", "/autoscale > /var/log/autoscale.log"]
