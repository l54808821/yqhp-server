package jsscript

import (
	"encoding/json"
	"fmt"
	"strings"

	"yqhp/workflow-engine/internal/keyword"

	"github.com/dop251/goja"
)

// setupEnvironment sets up the JavaScript execution environment.
func setupEnvironment(vm *goja.Runtime, execCtx *keyword.ExecutionContext, consoleLogs *[]string) error {
	// Setup console object
	if err := setupConsole(vm, consoleLogs); err != nil {
		return err
	}

	// Setup request object
	if err := setupRequest(vm, execCtx); err != nil {
		return err
	}

	// Setup response object
	if err := setupResponse(vm, execCtx); err != nil {
		return err
	}

	// Setup pm object (Postman-like API)
	if err := setupPM(vm, execCtx, consoleLogs); err != nil {
		return err
	}

	return nil
}

// setupConsole sets up the console object.
func setupConsole(vm *goja.Runtime, logs *[]string) error {
	console := vm.NewObject()

	logFn := func(level string) func(call goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			args := make([]string, len(call.Arguments))
			for i, arg := range call.Arguments {
				args[i] = fmt.Sprintf("%v", arg.Export())
			}
			msg := fmt.Sprintf("[%s] %s", level, joinArgs(args))
			*logs = append(*logs, msg)
			return goja.Undefined()
		}
	}

	console.Set("log", logFn("LOG"))
	console.Set("info", logFn("INFO"))
	console.Set("warn", logFn("WARN"))
	console.Set("error", logFn("ERROR"))
	console.Set("debug", logFn("DEBUG"))

	vm.Set("console", console)
	return nil
}

// setupRequest sets up the request object.
func setupRequest(vm *goja.Runtime, execCtx *keyword.ExecutionContext) error {
	request := vm.NewObject()

	// Get request data from metadata if available
	if reqData, ok := execCtx.GetMetadata("request"); ok {
		if reqMap, ok := reqData.(map[string]any); ok {
			for k, v := range reqMap {
				request.Set(k, v)
			}
		}
	}

	vm.Set("request", request)
	return nil
}

// setupResponse sets up the response object.
func setupResponse(vm *goja.Runtime, execCtx *keyword.ExecutionContext) error {
	response := vm.NewObject()

	resp := execCtx.GetResponse()
	if resp != nil {
		response.Set("statusCode", resp.Status)
		response.Set("status", resp.Status)
		response.Set("headers", resp.Headers)
		response.Set("body", resp.Body)
		response.Set("duration", resp.Duration)

		// Parse body as JSON if possible
		if resp.Body != "" {
			var jsonData any
			if err := json.Unmarshal([]byte(resp.Body), &jsonData); err == nil {
				response.Set("json", func() any { return jsonData })
			}
		}
	}

	vm.Set("response", response)
	return nil
}

// setupPM sets up the pm object (Postman-like API).
func setupPM(vm *goja.Runtime, execCtx *keyword.ExecutionContext, consoleLogs *[]string) error {
	pm := vm.NewObject()

	// pm.variables - temporary variables
	variables := vm.NewObject()
	variables.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		name := call.Arguments[0].String()
		if val, ok := execCtx.GetVariable(name); ok {
			return vm.ToValue(val)
		}
		return goja.Undefined()
	})
	variables.Set("set", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		name := call.Arguments[0].String()
		value := call.Arguments[1].Export()
		execCtx.SetVariable(name, value)
		return goja.Undefined()
	})
	variables.Set("has", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue(false)
		}
		name := call.Arguments[0].String()
		_, ok := execCtx.GetVariable(name)
		return vm.ToValue(ok)
	})
	pm.Set("variables", variables)

	// pm.environment - environment variables
	environment := vm.NewObject()
	environment.Set("get", variables.Get("get"))
	environment.Set("set", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		name := call.Arguments[0].String()
		value := call.Arguments[1].Export()
		execCtx.SetVariable(name, value)
		// Mark as environment variable for persistence
		execCtx.SetMetadata(fmt.Sprintf("env_var_%s", name), value)
		return goja.Undefined()
	})
	environment.Set("has", variables.Get("has"))
	pm.Set("environment", environment)

	// pm.test - test function for assertions
	testResults := make([]map[string]any, 0)
	pm.Set("test", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		testName := call.Arguments[0].String()
		testFn, ok := goja.AssertFunction(call.Arguments[1])
		if !ok {
			return goja.Undefined()
		}

		// Execute test function
		_, err := testFn(goja.Undefined())
		result := map[string]any{
			"name":   testName,
			"passed": err == nil,
			"error":  "",
		}
		if err != nil {
			result["error"] = err.Error()
			*consoleLogs = append(*consoleLogs, fmt.Sprintf("[TEST FAIL] %s: %v", testName, err))
		} else {
			*consoleLogs = append(*consoleLogs, fmt.Sprintf("[TEST PASS] %s", testName))
		}
		testResults = append(testResults, result)
		execCtx.SetMetadata("test_results", testResults)
		return goja.Undefined()
	})

	// pm.expect - chai-like expect function
	pm.Set("expect", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		actual := call.Arguments[0].Export()
		return createExpectObject(vm, actual)
	})

	// pm.response - shortcut to response
	pm.Set("response", vm.Get("response"))

	vm.Set("pm", pm)
	return nil
}

// createExpectObject creates a chai-like expect object.
func createExpectObject(vm *goja.Runtime, actual any) goja.Value {
	expect := vm.NewObject()

	// to.be chain
	to := vm.NewObject()
	be := vm.NewObject()

	// to.equal
	to.Set("equal", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		expected := call.Arguments[0].Export()
		if !deepEqual(actual, expected) {
			panic(vm.NewGoError(fmt.Errorf("expected %v to equal %v", actual, expected)))
		}
		return goja.Undefined()
	})

	// to.eql (deep equal)
	to.Set("eql", to.Get("equal"))

	// to.be.true
	be.Set("true", func(call goja.FunctionCall) goja.Value {
		if actual != true {
			panic(vm.NewGoError(fmt.Errorf("expected %v to be true", actual)))
		}
		return goja.Undefined()
	})

	// to.be.false
	be.Set("false", func(call goja.FunctionCall) goja.Value {
		if actual != false {
			panic(vm.NewGoError(fmt.Errorf("expected %v to be false", actual)))
		}
		return goja.Undefined()
	})

	// to.be.null
	be.Set("null", func(call goja.FunctionCall) goja.Value {
		if actual != nil {
			panic(vm.NewGoError(fmt.Errorf("expected %v to be null", actual)))
		}
		return goja.Undefined()
	})

	// to.be.undefined
	be.Set("undefined", func(call goja.FunctionCall) goja.Value {
		if actual != nil {
			panic(vm.NewGoError(fmt.Errorf("expected %v to be undefined", actual)))
		}
		return goja.Undefined()
	})

	// to.be.above
	be.Set("above", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		expected := call.Arguments[0].Export()
		if !isAbove(actual, expected) {
			panic(vm.NewGoError(fmt.Errorf("expected %v to be above %v", actual, expected)))
		}
		return goja.Undefined()
	})

	// to.be.below
	be.Set("below", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		expected := call.Arguments[0].Export()
		if !isBelow(actual, expected) {
			panic(vm.NewGoError(fmt.Errorf("expected %v to be below %v", actual, expected)))
		}
		return goja.Undefined()
	})

	// to.be.a / to.be.an
	be.Set("a", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		expectedType := call.Arguments[0].String()
		if !isType(actual, expectedType) {
			panic(vm.NewGoError(fmt.Errorf("expected %v to be a %s", actual, expectedType)))
		}
		return goja.Undefined()
	})
	be.Set("an", be.Get("a"))

	to.Set("be", be)

	// to.have chain
	have := vm.NewObject()

	// to.have.property
	have.Set("property", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		propName := call.Arguments[0].String()
		if !hasProperty(actual, propName) {
			panic(vm.NewGoError(fmt.Errorf("expected %v to have property '%s'", actual, propName)))
		}
		return goja.Undefined()
	})

	// to.have.length
	have.Set("length", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		expectedLen := toInt(call.Arguments[0].Export())
		actualLen := getLength(actual)
		if actualLen != expectedLen {
			panic(vm.NewGoError(fmt.Errorf("expected %v to have length %d, got %d", actual, expectedLen, actualLen)))
		}
		return goja.Undefined()
	})

	// to.have.status (for response)
	have.Set("status", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		expectedStatus := toInt(call.Arguments[0].Export())
		actualStatus := getStatus(actual)
		if actualStatus != expectedStatus {
			panic(vm.NewGoError(fmt.Errorf("expected status %d, got %d", expectedStatus, actualStatus)))
		}
		return goja.Undefined()
	})

	to.Set("have", have)

	// to.include
	to.Set("include", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		expected := call.Arguments[0].Export()
		if !includes(actual, expected) {
			panic(vm.NewGoError(fmt.Errorf("expected %v to include %v", actual, expected)))
		}
		return goja.Undefined()
	})
	to.Set("contain", to.Get("include"))

	expect.Set("to", to)

	return expect
}

// Helper functions

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

func deepEqual(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func isAbove(actual, expected any) bool {
	a := toFloat64(actual)
	b := toFloat64(expected)
	return a > b
}

func isBelow(actual, expected any) bool {
	a := toFloat64(actual)
	b := toFloat64(expected)
	return a < b
}

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case float32:
		return float64(n)
	case float64:
		return n
	default:
		return 0
	}
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float32:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func isType(v any, typeName string) bool {
	switch typeName {
	case "string":
		_, ok := v.(string)
		return ok
	case "number":
		switch v.(type) {
		case int, int32, int64, float32, float64:
			return true
		}
		return false
	case "boolean", "bool":
		_, ok := v.(bool)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	default:
		return false
	}
}

func hasProperty(v any, prop string) bool {
	if m, ok := v.(map[string]any); ok {
		_, exists := m[prop]
		return exists
	}
	return false
}

func getLength(v any) int {
	switch val := v.(type) {
	case string:
		return len(val)
	case []any:
		return len(val)
	case map[string]any:
		return len(val)
	default:
		return 0
	}
}

func getStatus(v any) int {
	if m, ok := v.(map[string]any); ok {
		if status, ok := m["statusCode"]; ok {
			return toInt(status)
		}
		if status, ok := m["status"]; ok {
			return toInt(status)
		}
	}
	return 0
}

func includes(container, item any) bool {
	switch c := container.(type) {
	case string:
		if s, ok := item.(string); ok {
			return strings.Contains(c, s)
		}
	case []any:
		for _, v := range c {
			if deepEqual(v, item) {
				return true
			}
		}
	case map[string]any:
		if key, ok := item.(string); ok {
			_, exists := c[key]
			return exists
		}
	}
	return false
}
