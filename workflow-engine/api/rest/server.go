// Package rest provides the REST API server for the workflow execution engine.
// Requirements: 7.5
package rest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"yqhp/workflow-engine/internal/master"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"
)

// Server represents the REST API server.
type Server struct {
	app      *fiber.App
	master   master.Master
	registry master.SlaveRegistry
	config   *Config

	// 任务和命令队列管理
	taskQueues      map[string]chan *TaskAssignment // slaveID -> task queue
	taskQueuesMu    sync.RWMutex
	commandQueues   map[string]chan *ControlCommand // slaveID -> command queue
	commandQueuesMu sync.RWMutex
}

// Config holds the configuration for the REST API server.
type Config struct {
	// Address is the address to listen on (e.g., ":8080").
	Address string `yaml:"address"`

	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration `yaml:"read_timeout"`

	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration `yaml:"write_timeout"`

	// EnableCORS enables Cross-Origin Resource Sharing.
	EnableCORS bool `yaml:"enable_cors"`

	// EnableSwagger enables Swagger documentation endpoint.
	EnableSwagger bool `yaml:"enable_swagger"`

	// EnableMetrics enables the /metrics endpoint.
	EnableMetrics bool `yaml:"enable_metrics"`

	// EnableWebSocket enables WebSocket support.
	EnableWebSocket bool `yaml:"enable_websocket"`

	// Auth holds authentication configuration.
	Auth *AuthConfig `yaml:"auth,omitempty"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	// Enabled enables authentication.
	Enabled bool `yaml:"enabled"`

	// Type is the authentication type (e.g., "api_key", "jwt").
	Type string `yaml:"type"`

	// APIKey is the API key for api_key authentication.
	APIKey string `yaml:"api_key,omitempty"`

	// JWTSecret is the secret for JWT authentication.
	JWTSecret string `yaml:"jwt_secret,omitempty"`
}

// DefaultConfig returns a default server configuration.
func DefaultConfig() *Config {
	return &Config{
		Address:         ":8080",
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		EnableCORS:      true,
		EnableSwagger:   false,
		EnableMetrics:   true,
		EnableWebSocket: true,
		Auth:            nil,
	}
}

// NewServer creates a new REST API server.
// Requirements: 7.5
func NewServer(m master.Master, registry master.SlaveRegistry, config *Config) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	app := fiber.New(fiber.Config{
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		ErrorHandler: customErrorHandler,
		AppName:      "Workflow Engine API",
	})

	server := &Server{
		app:           app,
		master:        m,
		registry:      registry,
		config:        config,
		taskQueues:    make(map[string]chan *TaskAssignment),
		commandQueues: make(map[string]chan *ControlCommand),
	}

	server.setupMiddleware()
	server.setupRoutes()

	return server
}

// setupMiddleware configures middleware for the server.
func (s *Server) setupMiddleware() {
	// Recovery middleware - recovers from panics
	s.app.Use(fiberrecover.New(fiberrecover.Config{
		EnableStackTrace: true,
	}))

	// Logger middleware - logs HTTP requests
	s.app.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))

	// CORS middleware
	if s.config.EnableCORS {
		s.app.Use(cors.New(cors.Config{
			AllowOrigins:     "*",
			AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
			AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-API-Key",
			AllowCredentials: false,
			MaxAge:           86400,
		}))
	}

	// Authentication middleware (if enabled)
	if s.config.Auth != nil && s.config.Auth.Enabled {
		s.app.Use(s.authMiddleware())
	}
}

// authMiddleware returns the authentication middleware.
func (s *Server) authMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip authentication for health check endpoints
		path := c.Path()
		if path == "/health" || path == "/ready" || path == "/api/v1/health" || path == "/api/v1/ready" {
			return c.Next()
		}

		if s.config.Auth == nil || !s.config.Auth.Enabled {
			return c.Next()
		}

		switch s.config.Auth.Type {
		case "api_key":
			return s.apiKeyAuth(c)
		case "jwt":
			return s.jwtAuth(c)
		default:
			return c.Next()
		}
	}
}

// apiKeyAuth validates API key authentication.
func (s *Server) apiKeyAuth(c *fiber.Ctx) error {
	apiKey := c.Get("X-API-Key")
	if apiKey == "" {
		apiKey = c.Query("api_key")
	}

	if apiKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error:   "unauthorized",
			Message: "API key is required",
		})
	}

	if apiKey != s.config.Auth.APIKey {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error:   "unauthorized",
			Message: "Invalid API key",
		})
	}

	return c.Next()
}

// jwtAuth validates JWT authentication.
func (s *Server) jwtAuth(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error:   "unauthorized",
			Message: "Authorization header is required",
		})
	}

	// For now, just check if the header is present
	// In a real implementation, we would validate the JWT token
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{
			Error:   "unauthorized",
			Message: "Invalid authorization header format",
		})
	}

	return c.Next()
}

// setupRoutes configures the API routes.
func (s *Server) setupRoutes() {
	// Health check endpoints
	s.app.Get("/health", s.healthCheck)
	s.app.Get("/ready", s.readyCheck)

	// API v1 routes
	api := s.app.Group("/api/v1")

	// Health check endpoints (also under /api/v1)
	api.Get("/health", s.healthCheck)
	api.Get("/ready", s.readyCheck)

	// Workflow routes
	api.Post("/workflows", s.submitWorkflow)
	api.Get("/workflows/:id", s.getWorkflow)
	api.Delete("/workflows/:id", s.stopWorkflow)

	// Execution routes
	api.Get("/executions", s.listExecutions)
	api.Get("/executions/:id", s.getExecution)
	api.Post("/executions/:id/pause", s.pauseExecution)
	api.Post("/executions/:id/resume", s.resumeExecution)
	api.Post("/executions/:id/scale", s.scaleExecution)
	api.Delete("/executions/:id", s.stopExecution)

	// Metrics routes
	api.Get("/executions/:id/metrics", s.getMetrics)

	// Slave routes
	api.Get("/slaves", s.listSlaves)
	api.Get("/slaves/:id", s.getSlave)
	api.Post("/slaves/:id/drain", s.drainSlave)

	// Slave 通信路由 (HTTP REST API)
	api.Post("/slaves/register", s.registerSlave)
	api.Post("/slaves/:id/heartbeat", s.slaveHeartbeat)
	api.Get("/slaves/:id/tasks", s.getSlaveTasks)
	api.Post("/slaves/:id/unregister", s.unregisterSlave)

	// 任务结果路由
	api.Post("/tasks/:id/result", s.receiveTaskResult)

	// 指标报告路由
	api.Post("/executions/:id/metrics/report", s.receiveMetricsReport)

	// 执行路由（统一执行入口，支持单步和流程执行，支持 SSE 和阻塞）
	s.setupExecuteRoutes()

	// WebSocket routes
	s.setupWebSocketRoutes()
}

// Start starts the REST API server.
func (s *Server) Start() error {
	return s.app.Listen(s.config.Address)
}

// StartWithContext starts the REST API server with context support.
func (s *Server) StartWithContext(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- s.app.Listen(s.config.Address)
	}()

	select {
	case <-ctx.Done():
		return s.Shutdown()
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

// ShutdownWithTimeout gracefully shuts down the server with a timeout.
func (s *Server) ShutdownWithTimeout(timeout time.Duration) error {
	return s.app.ShutdownWithTimeout(timeout)
}

// App returns the underlying Fiber app.
func (s *Server) App() *fiber.App {
	return s.app
}

// customErrorHandler handles errors returned by handlers.
func customErrorHandler(c *fiber.Ctx, err error) error {
	// Default to 500 Internal Server Error
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	// Check if it's a Fiber error
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(ErrorResponse{
		Error:   fmt.Sprintf("error_%d", code),
		Message: message,
	})
}
