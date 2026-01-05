//go:build tools
// +build tools

package tools

// This file declares tool dependencies that are used by the project.
// These imports ensure the dependencies are tracked in go.mod.

import (
	_ "github.com/gofiber/fiber/v2"
	_ "github.com/leanovate/gopter"
	_ "github.com/stretchr/testify/assert"
	_ "google.golang.org/grpc"
	_ "google.golang.org/protobuf/proto"
	_ "gopkg.in/yaml.v3"
)
