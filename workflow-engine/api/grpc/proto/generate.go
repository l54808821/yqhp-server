//go:build ignore
// +build ignore

// This file is used to generate Go code from protobuf definitions.
// Run: go generate ./api/grpc/proto/...

package proto

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative workflow.proto
