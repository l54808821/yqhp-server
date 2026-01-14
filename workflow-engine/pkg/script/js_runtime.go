// Package script 提供 JavaScript 脚本执行运行时。
package script

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// JSRuntime JavaScript 运行时封装
type JSRuntime struct {
	vm          *goja.Runtime
	consoleLogs []string
	logMu       sync.Mutex
	variables   map[string]interface{}
	envVars     map[string]interface{}
	httpClient  *http.Client
	prevResult  interface{}
	request     interface{}
	response    interface{}
}

// ScriptResult 脚本执行结果
type ScriptResult struct {
	Value       interface{}            `json:"value"`
	ConsoleLogs []string               `json:"console_logs"`
	Variables   map[string]interface{} `json:"variables"`
	EnvVars     map[string]interface{} `json:"env_vars"`
	Error       error                  `json:"-"`
	ErrorMsg    string                 `json:"error,omitempty"`
}

// JSRuntimeConfig 运行时配置
type JSRuntimeConfig struct {
	Variables  map[string]interface{} // 初始变量
	EnvVars    map[string]interface{} // 环境变量
	PrevResult interface{}            // 上一步骤结果
	Request    interface{}            // 当前请求信息
	Response   interface{}            // 上一个 HTTP 响应
	HTTPClient *http.Client           // HTTP 客户端
}

// NewJSRuntime 创建新的 JS 运行时
func NewJSRuntime(config *JSRuntimeConfig) *JSRuntime {
	if config == nil {
		config = &JSRuntimeConfig{}
	}

	rt := &JSRuntime{
		vm:          goja.New(),
		consoleLogs: make([]string, 0),
		variables:   make(map[string]interface{}),
		envVars:     make(map[string]interface{}),
		httpClient:  config.HTTPClient,
		prevResult:  config.PrevResult,
		request:     config.Request,
		response:    config.Response,
	}

	// 复制初始变量
	if config.Variables != nil {
		for k, v := range config.Variables {
			rt.variables[k] = v
		}
	}

	// 复制环境变量
	if config.EnvVars != nil {
		for k, v := range config.EnvVars {
			rt.envVars[k] = v
		}
	}

	// 设置默认 HTTP 客户端
	if rt.httpClient == nil {
		rt.httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	// 初始化运行时环境
	rt.setupConsole()
	rt.setupPMAPI()
	rt.setupUtils()

	return rt
}

// Execute 执行脚本
func (r *JSRuntime) Execute(script string, timeout time.Duration) (*ScriptResult, error) {
	result := &ScriptResult{
		ConsoleLogs: make([]string, 0),
		Variables:   make(map[string]interface{}),
		EnvVars:     make(map[string]interface{}),
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 设置中断处理
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			r.vm.Interrupt("脚本执行超时")
		case <-done:
		}
	}()

	// 执行脚本
	val, err := r.vm.RunString(script)
	close(done)

	// 收集结果
	r.logMu.Lock()
	result.ConsoleLogs = append(result.ConsoleLogs, r.consoleLogs...)
	r.logMu.Unlock()

	// 复制变量
	for k, v := range r.variables {
		result.Variables[k] = v
	}
	for k, v := range r.envVars {
		result.EnvVars[k] = v
	}

	if err != nil {
		// 检查是否是超时
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("脚本执行超时")
			result.ErrorMsg = "脚本执行超时"
		} else {
			result.Error = err
			result.ErrorMsg = err.Error()
		}
		return result, err
	}

	// 导出返回值
	if val != nil && !goja.IsUndefined(val) && !goja.IsNull(val) {
		result.Value = val.Export()
	}

	return result, nil
}

// setupConsole 设置 console 对象
func (r *JSRuntime) setupConsole() {
	console := r.vm.NewObject()

	// console.log
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		r.appendLog("LOG", call.Arguments)
		return goja.Undefined()
	})

	// console.warn
	console.Set("warn", func(call goja.FunctionCall) goja.Value {
		r.appendLog("WARN", call.Arguments)
		return goja.Undefined()
	})

	// console.error
	console.Set("error", func(call goja.FunctionCall) goja.Value {
		r.appendLog("ERROR", call.Arguments)
		return goja.Undefined()
	})

	// console.info
	console.Set("info", func(call goja.FunctionCall) goja.Value {
		r.appendLog("INFO", call.Arguments)
		return goja.Undefined()
	})

	r.vm.Set("console", console)
}

// appendLog 添加日志
func (r *JSRuntime) appendLog(level string, args []goja.Value) {
	r.logMu.Lock()
	defer r.logMu.Unlock()

	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = r.formatValue(arg)
	}

	logLine := fmt.Sprintf("[%s] %s", level, strings.Join(parts, " "))
	r.consoleLogs = append(r.consoleLogs, logLine)
}

// formatValue 格式化值为字符串
func (r *JSRuntime) formatValue(val goja.Value) string {
	if val == nil || goja.IsUndefined(val) {
		return "undefined"
	}
	if goja.IsNull(val) {
		return "null"
	}

	exported := val.Export()
	switch v := exported.(type) {
	case string:
		return v
	case map[string]interface{}, []interface{}:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// setupPMAPI 设置 pm 对象 (Postman 风格 API)
func (r *JSRuntime) setupPMAPI() {
	pm := r.vm.NewObject()

	// pm.environment
	environment := r.vm.NewObject()
	environment.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		if val, ok := r.envVars[key]; ok {
			return r.vm.ToValue(val)
		}
		return goja.Undefined()
	})
	environment.Set("set", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		val := call.Arguments[1].Export()
		r.envVars[key] = val
		return goja.Undefined()
	})
	environment.Set("has", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue(false)
		}
		key := call.Arguments[0].String()
		_, ok := r.envVars[key]
		return r.vm.ToValue(ok)
	})
	environment.Set("unset", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		delete(r.envVars, key)
		return goja.Undefined()
	})
	environment.Set("clear", func(call goja.FunctionCall) goja.Value {
		r.envVars = make(map[string]interface{})
		return goja.Undefined()
	})
	environment.Set("toObject", func(call goja.FunctionCall) goja.Value {
		return r.vm.ToValue(r.envVars)
	})
	pm.Set("environment", environment)

	// pm.variables
	variables := r.vm.NewObject()
	variables.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		if val, ok := r.variables[key]; ok {
			return r.vm.ToValue(val)
		}
		return goja.Undefined()
	})
	variables.Set("set", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		val := call.Arguments[1].Export()
		r.variables[key] = val
		return goja.Undefined()
	})
	variables.Set("has", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue(false)
		}
		key := call.Arguments[0].String()
		_, ok := r.variables[key]
		return r.vm.ToValue(ok)
	})
	variables.Set("unset", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		delete(r.variables, key)
		return goja.Undefined()
	})
	variables.Set("clear", func(call goja.FunctionCall) goja.Value {
		r.variables = make(map[string]interface{})
		return goja.Undefined()
	})
	variables.Set("toObject", func(call goja.FunctionCall) goja.Value {
		return r.vm.ToValue(r.variables)
	})
	pm.Set("variables", variables)

	// pm.response
	r.setupPMResponse(pm)

	// pm.request
	r.setupPMRequest(pm)

	// pm.sendRequest
	pm.Set("sendRequest", r.createSendRequestFunc())

	r.vm.Set("pm", pm)
}

// setupPMResponse 设置 pm.response 对象
func (r *JSRuntime) setupPMResponse(pm *goja.Object) {
	response := r.vm.NewObject()

	if r.response != nil {
		// 如果有响应数据，设置相关属性
		if respMap, ok := r.response.(map[string]interface{}); ok {
			if code, ok := respMap["status_code"].(int); ok {
				response.Set("code", code)
			}
			if status, ok := respMap["status"].(string); ok {
				response.Set("status", status)
			}
			if headers, ok := respMap["headers"].(map[string]interface{}); ok {
				response.Set("headers", r.vm.ToValue(headers))
			}
			if body, ok := respMap["body"]; ok {
				response.Set("body", r.vm.ToValue(body))
			}
			if bodyRaw, ok := respMap["body_raw"].(string); ok {
				response.Set("text", func(call goja.FunctionCall) goja.Value {
					return r.vm.ToValue(bodyRaw)
				})
				response.Set("json", func(call goja.FunctionCall) goja.Value {
					var result interface{}
					if err := json.Unmarshal([]byte(bodyRaw), &result); err != nil {
						panic(r.vm.NewGoError(fmt.Errorf("JSON 解析错误: %v", err)))
					}
					return r.vm.ToValue(result)
				})
			}
		}
	} else {
		// 默认空响应
		response.Set("code", 0)
		response.Set("status", "")
		response.Set("headers", r.vm.NewObject())
		response.Set("body", goja.Undefined())
		response.Set("text", func(call goja.FunctionCall) goja.Value {
			return r.vm.ToValue("")
		})
		response.Set("json", func(call goja.FunctionCall) goja.Value {
			return goja.Undefined()
		})
	}

	pm.Set("response", response)
}

// setupPMRequest 设置 pm.request 对象
func (r *JSRuntime) setupPMRequest(pm *goja.Object) {
	request := r.vm.NewObject()

	if r.request != nil {
		if reqMap, ok := r.request.(map[string]interface{}); ok {
			if method, ok := reqMap["method"].(string); ok {
				request.Set("method", method)
			}
			if urlStr, ok := reqMap["url"].(string); ok {
				request.Set("url", urlStr)
			}
			if headers, ok := reqMap["headers"].(map[string]interface{}); ok {
				request.Set("headers", r.vm.ToValue(headers))
			}
			if body, ok := reqMap["body"]; ok {
				request.Set("body", r.vm.ToValue(body))
			}
		}
	} else {
		request.Set("method", "")
		request.Set("url", "")
		request.Set("headers", r.vm.NewObject())
		request.Set("body", goja.Undefined())
	}

	pm.Set("request", request)
}

// createSendRequestFunc 创建 pm.sendRequest 函数
func (r *JSRuntime) createSendRequestFunc() func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(r.vm.NewGoError(fmt.Errorf("pm.sendRequest 需要 url 和 callback 参数")))
		}

		// 获取 URL 或请求配置
		var reqURL string
		var method string = "GET"
		var headers map[string]string
		var body string

		arg0 := call.Arguments[0].Export()
		switch v := arg0.(type) {
		case string:
			reqURL = v
		case map[string]interface{}:
			if u, ok := v["url"].(string); ok {
				reqURL = u
			}
			if m, ok := v["method"].(string); ok {
				method = strings.ToUpper(m)
			}
			if h, ok := v["headers"].(map[string]interface{}); ok {
				headers = make(map[string]string)
				for k, val := range h {
					headers[k] = fmt.Sprintf("%v", val)
				}
			}
			if b, ok := v["body"].(string); ok {
				body = b
			}
		default:
			panic(r.vm.NewGoError(fmt.Errorf("pm.sendRequest 第一个参数必须是 URL 字符串或请求配置对象")))
		}

		// 获取回调函数
		callback, ok := goja.AssertFunction(call.Arguments[1])
		if !ok {
			panic(r.vm.NewGoError(fmt.Errorf("pm.sendRequest 第二个参数必须是回调函数")))
		}

		// 执行 HTTP 请求
		go func() {
			var req *http.Request
			var err error

			if body != "" {
				req, err = http.NewRequest(method, reqURL, strings.NewReader(body))
			} else {
				req, err = http.NewRequest(method, reqURL, nil)
			}

			if err != nil {
				r.callCallback(callback, err, nil)
				return
			}

			// 设置请求头
			for k, v := range headers {
				req.Header.Set(k, v)
			}

			resp, err := r.httpClient.Do(req)
			if err != nil {
				r.callCallback(callback, err, nil)
				return
			}
			defer resp.Body.Close()

			// 构建响应对象
			respObj := map[string]interface{}{
				"code":   resp.StatusCode,
				"status": resp.Status,
			}

			// 读取响应头
			respHeaders := make(map[string]string)
			for k, v := range resp.Header {
				if len(v) > 0 {
					respHeaders[k] = v[0]
				}
			}
			respObj["headers"] = respHeaders

			r.callCallback(callback, nil, respObj)
		}()

		return goja.Undefined()
	}
}

// callCallback 调用回调函数
func (r *JSRuntime) callCallback(callback goja.Callable, err error, response interface{}) {
	var errVal goja.Value = goja.Null()
	var respVal goja.Value = goja.Null()

	if err != nil {
		errVal = r.vm.ToValue(err.Error())
	}
	if response != nil {
		respVal = r.vm.ToValue(response)
	}

	_, _ = callback(goja.Undefined(), errVal, respVal)
}

// setupUtils 设置工具方法
func (r *JSRuntime) setupUtils() {
	// atob - Base64 解码
	r.vm.Set("atob", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue("")
		}
		encoded := call.Arguments[0].String()
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			panic(r.vm.NewGoError(fmt.Errorf("Base64 解码错误: %v", err)))
		}
		return r.vm.ToValue(string(decoded))
	})

	// btoa - Base64 编码
	r.vm.Set("btoa", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue("")
		}
		str := call.Arguments[0].String()
		encoded := base64.StdEncoding.EncodeToString([]byte(str))
		return r.vm.ToValue(encoded)
	})

	// encodeURIComponent
	r.vm.Set("encodeURIComponent", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue("")
		}
		str := call.Arguments[0].String()
		return r.vm.ToValue(url.QueryEscape(str))
	})

	// decodeURIComponent
	r.vm.Set("decodeURIComponent", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue("")
		}
		str := call.Arguments[0].String()
		decoded, err := url.QueryUnescape(str)
		if err != nil {
			panic(r.vm.NewGoError(fmt.Errorf("URL 解码错误: %v", err)))
		}
		return r.vm.ToValue(decoded)
	})

	// encodeURI
	r.vm.Set("encodeURI", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue("")
		}
		str := call.Arguments[0].String()
		// encodeURI 不编码 :/?#[]@!$&'()*+,;=
		return r.vm.ToValue(url.PathEscape(str))
	})

	// decodeURI
	r.vm.Set("decodeURI", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue("")
		}
		str := call.Arguments[0].String()
		decoded, err := url.PathUnescape(str)
		if err != nil {
			panic(r.vm.NewGoError(fmt.Errorf("URI 解码错误: %v", err)))
		}
		return r.vm.ToValue(decoded)
	})

	// setTimeout (简化版，同步执行)
	r.vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		fn, ok := goja.AssertFunction(call.Arguments[0])
		if !ok {
			return goja.Undefined()
		}
		// 简化实现：直接执行
		fn(goja.Undefined())
		return r.vm.ToValue(1)
	})

	// crypto 对象
	r.setupCrypto()
}

// setupCrypto 设置 crypto 对象
func (r *JSRuntime) setupCrypto() {
	crypto := r.vm.NewObject()

	// crypto.md5
	crypto.Set("md5", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue("")
		}
		str := call.Arguments[0].String()
		hash := md5Hash(str)
		return r.vm.ToValue(hash)
	})

	// crypto.sha1
	crypto.Set("sha1", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue("")
		}
		str := call.Arguments[0].String()
		hash := sha1Hash(str)
		return r.vm.ToValue(hash)
	})

	// crypto.sha256
	crypto.Set("sha256", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue("")
		}
		str := call.Arguments[0].String()
		hash := sha256Hash(str)
		return r.vm.ToValue(hash)
	})

	// crypto.sha512
	crypto.Set("sha512", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue("")
		}
		str := call.Arguments[0].String()
		hash := sha512Hash(str)
		return r.vm.ToValue(hash)
	})

	r.vm.Set("crypto", crypto)
}

// GetConsoleLogs 获取控制台日志
func (r *JSRuntime) GetConsoleLogs() []string {
	r.logMu.Lock()
	defer r.logMu.Unlock()
	logs := make([]string, len(r.consoleLogs))
	copy(logs, r.consoleLogs)
	return logs
}

// GetVariables 获取变量
func (r *JSRuntime) GetVariables() map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range r.variables {
		result[k] = v
	}
	return result
}

// GetEnvVars 获取环境变量
func (r *JSRuntime) GetEnvVars() map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range r.envVars {
		result[k] = v
	}
	return result
}

// SetVariable 设置变量
func (r *JSRuntime) SetVariable(key string, value interface{}) {
	r.variables[key] = value
}

// SetEnvVar 设置环境变量
func (r *JSRuntime) SetEnvVar(key string, value interface{}) {
	r.envVars[key] = value
}
