package keyword

import (
	"sync"
)

// ExecutionContext holds the execution context for keywords.
// It provides access to variables, response data, and other execution state.
type ExecutionContext struct {
	mu        sync.RWMutex
	variables map[string]any // Variable storage
	response  *ResponseData  // Current response data
	metadata  map[string]any // Additional metadata
}

// ResponseData represents the response from a step execution.
type ResponseData struct {
	Status   int               `json:"status"`            // Status code
	Headers  map[string]string `json:"headers,omitempty"` // Response headers
	Body     string            `json:"body,omitempty"`    // Raw response body
	Data     any               `json:"data,omitempty"`    // Parsed data (JSON/XML/list)
	Duration int64             `json:"duration"`          // Execution duration in ms
	Error    string            `json:"error,omitempty"`   // Error message if any
}

// NewExecutionContext creates a new execution context.
func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		variables: make(map[string]any),
		metadata:  make(map[string]any),
	}
}

// NewExecutionContextWithVars creates a new execution context with initial variables.
func NewExecutionContextWithVars(vars map[string]any) *ExecutionContext {
	ctx := NewExecutionContext()
	for k, v := range vars {
		ctx.variables[k] = v
	}
	return ctx
}

// SetVariable sets a variable in the context.
func (c *ExecutionContext) SetVariable(name string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.variables[name] = value
}

// GetVariable gets a variable from the context.
func (c *ExecutionContext) GetVariable(name string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.variables[name]
	return val, ok
}

// GetVariables returns a copy of all variables.
func (c *ExecutionContext) GetVariables() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]any, len(c.variables))
	for k, v := range c.variables {
		result[k] = v
	}
	return result
}

// DeleteVariable deletes a variable from the context.
func (c *ExecutionContext) DeleteVariable(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.variables, name)
}

// SetResponse sets the current response data.
func (c *ExecutionContext) SetResponse(resp *ResponseData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.response = resp
	// Also set response as a variable for ${response.xxx} access
	c.variables["response"] = resp
}

// GetResponse gets the current response data.
func (c *ExecutionContext) GetResponse() *ResponseData {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.response
}

// SetMetadata sets metadata in the context.
func (c *ExecutionContext) SetMetadata(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMetadata gets metadata from the context.
func (c *ExecutionContext) GetMetadata(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.metadata[key]
	return val, ok
}

// Clone creates a shallow copy of the execution context.
// Useful for parallel execution with variable isolation.
func (c *ExecutionContext) Clone() *ExecutionContext {
	c.mu.RLock()
	defer c.mu.RUnlock()

	newCtx := &ExecutionContext{
		variables: make(map[string]any, len(c.variables)),
		metadata:  make(map[string]any, len(c.metadata)),
		response:  c.response,
	}
	for k, v := range c.variables {
		newCtx.variables[k] = v
	}
	for k, v := range c.metadata {
		newCtx.metadata[k] = v
	}
	return newCtx
}

// Merge merges variables from another context into this one.
// Existing variables will be overwritten.
func (c *ExecutionContext) Merge(other *ExecutionContext) {
	if other == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	for k, v := range other.variables {
		c.variables[k] = v
	}
}
