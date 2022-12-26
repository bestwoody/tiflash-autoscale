# !!! PLEASE use protoc-gen-go@v1.3.0
# go install github.com/golang/protobuf/protoc-gen-go@v1.3.0
protoc     --go_out=plugins=grpc:.    supervisor.proto 
#protoc --go_out=.  \
#    --go-grpc_out=.  \
#    supervisor.proto
