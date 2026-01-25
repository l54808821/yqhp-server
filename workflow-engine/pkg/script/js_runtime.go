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
	rt.setupSimpleAPI()
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

// setupSimpleAPI 设置简洁版 API（env、vars、http、response）
func (r *JSRuntime) setupSimpleAPI() {
	// env 对象 - 环境变量操作
	env := r.vm.NewObject()
	env.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		if val, ok := r.envVars[key]; ok {
			return r.vm.ToValue(val)
		}
		return goja.Undefined()
	})
	env.Set("set", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		val := call.Arguments[1].Export()
		r.envVars[key] = val
		return goja.Undefined()
	})
	env.Set("has", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue(false)
		}
		key := call.Arguments[0].String()
		_, ok := r.envVars[key]
		return r.vm.ToValue(ok)
	})
	env.Set("del", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		delete(r.envVars, key)
		return goja.Undefined()
	})
	env.Set("all", func(call goja.FunctionCall) goja.Value {
		return r.vm.ToValue(r.envVars)
	})
	r.vm.Set("env", env)

	// vars 对象 - 临时变量操作
	vars := r.vm.NewObject()
	vars.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		if val, ok := r.variables[key]; ok {
			return r.vm.ToValue(val)
		}
		return goja.Undefined()
	})
	vars.Set("set", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		val := call.Arguments[1].Export()
		r.variables[key] = val
		return goja.Undefined()
	})
	vars.Set("has", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return r.vm.ToValue(false)
		}
		key := call.Arguments[0].String()
		_, ok := r.variables[key]
		return r.vm.ToValue(ok)
	})
	vars.Set("del", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		key := call.Arguments[0].String()
		delete(r.variables, key)
		return goja.Undefined()
	})
	vars.Set("all", func(call goja.FunctionCall) goja.Value {
		return r.vm.ToValue(r.variables)
	})
	r.vm.Set("vars", vars)

	// response 全局对象 - 上一步 HTTP 响应
	response := r.vm.NewObject()
	if r.response != nil {
		if respMap, ok := r.response.(map[string]interface{}); ok {
			if code, ok := respMap["statusCode"].(int); ok {
				response.Set("code", code)
			} else {
				response.Set("code", 0)
			}
			if status, ok := respMap["statusText"].(string); ok {
				response.Set("status", status)
			} else {
				response.Set("status", "")
			}
			if headers, ok := respMap["headers"].(map[string]interface{}); ok {
				response.Set("headers", r.vm.ToValue(headers))
			} else {
				response.Set("headers", r.vm.NewObject())
			}
			if body, ok := respMap["body"]; ok {
				response.Set("body", r.vm.ToValue(body))
			} else {
				response.Set("body", goja.Undefined())
			}
			// body 可能是 string 或者 JSON 对象，尝试获取字符串形式
			var bodyStr string
			if bs, ok := respMap["body"].(string); ok {
				bodyStr = bs
			}
			response.Set("text", bodyStr)
			response.Set("json", func(call goja.FunctionCall) goja.Value {
				if bodyStr == "" {
					return goja.Undefined()
				}
				var result interface{}
				if err := json.Unmarshal([]byte(bodyStr), &result); err != nil {
					panic(r.vm.NewGoError(fmt.Errorf("JSON 解析错误: %v", err)))
				}
				return r.vm.ToValue(result)
			})
		}
	} else {
		response.Set("code", 0)
		response.Set("status", "")
		response.Set("headers", r.vm.NewObject())
		response.Set("body", goja.Undefined())
		response.Set("text", "")
		response.Set("json", func(call goja.FunctionCall) goja.Value {
			return goja.Undefined()
		})
	}
	r.vm.Set("response", response)

	// http 对象 - HTTP 请求
	httpObj := r.vm.NewObject()

	// http.get(url, callback) 或 http.get(url, options, callback)
	httpObj.Set("get", func(call goja.FunctionCall) goja.Value {
		return r.executeHTTPRequest("GET", call)
	})

	// http.post(url, options, callback)
	httpObj.Set("post", func(call goja.FunctionCall) goja.Value {
		return r.executeHTTPRequest("POST", call)
	})

	// http.put(url, options, callback)
	httpObj.Set("put", func(call goja.FunctionCall) goja.Value {
		return r.executeHTTPRequest("PUT", call)
	})

	// http.delete(url, callback) 或 http.delete(url, options, callback)
	httpObj.Set("delete", func(call goja.FunctionCall) goja.Value {
		return r.executeHTTPRequest("DELETE", call)
	})

	// http.patch(url, options, callback)
	httpObj.Set("patch", func(call goja.FunctionCall) goja.Value {
		return r.executeHTTPRequest("PATCH", call)
	})

	// http.request(options, callback) - 通用请求方法
	httpObj.Set("request", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			panic(r.vm.NewGoError(fmt.Errorf("http.request 需要 options 和 callback 参数")))
		}

		options := call.Arguments[0].Export()
		optMap, ok := options.(map[string]interface{})
		if !ok {
			panic(r.vm.NewGoError(fmt.Errorf("http.request 第一个参数必须是配置对象")))
		}

		method := "GET"
		if m, ok := optMap["method"].(string); ok {
			method = strings.ToUpper(m)
		}

		// 构造新的调用参数
		newCall := goja.FunctionCall{
			This:      call.This,
			Arguments: call.Arguments,
		}
		return r.executeHTTPRequest(method, newCall)
	})

	r.vm.Set("http", httpObj)
}

// executeHTTPRequest 执行 HTTP 请求的通用方法
func (r *JSRuntime) executeHTTPRequest(method string, call goja.FunctionCall) goja.Value {
	if len(call.Arguments) < 2 {
		panic(r.vm.NewGoError(fmt.Errorf("http.%s 需要 url 和 callback 参数", strings.ToLower(method))))
	}

	var reqURL string
	var headers map[string]string
	var body string
	var callback goja.Callable
	var ok bool

	// 解析第一个参数（URL 或 options）
	arg0 := call.Arguments[0].Export()
	switch v := arg0.(type) {
	case string:
		reqURL = v
	case map[string]interface{}:
		if u, ok := v["url"].(string); ok {
			reqURL = u
		}
		if h, ok := v["headers"].(map[string]interface{}); ok {
			headers = make(map[string]string)
			for k, val := range h {
				headers[k] = fmt.Sprintf("%v", val)
			}
		}
		if b, ok := v["body"].(string); ok {
			body = b
		} else if b, ok := v["body"].(map[string]interface{}); ok {
			bodyBytes, _ := json.Marshal(b)
			body = string(bodyBytes)
		}
	default:
		panic(r.vm.NewGoError(fmt.Errorf("http.%s 第一个参数必须是 URL 字符串或配置对象", strings.ToLower(method))))
	}

	// 解析后续参数
	if len(call.Arguments) == 2 {
		// http.get(url, callback) 或 http.get(options, callback)
		callback, ok = goja.AssertFunction(call.Arguments[1])
		if !ok {
			panic(r.vm.NewGoError(fmt.Errorf("http.%s 最后一个参数必须是回调函数", strings.ToLower(method))))
		}
	} else if len(call.Arguments) >= 3 {
		// http.get(url, options, callback)
		if options, ok := call.Arguments[1].Export().(map[string]interface{}); ok {
			if h, ok := options["headers"].(map[string]interface{}); ok {
				if headers == nil {
					headers = make(map[string]string)
				}
				for k, val := range h {
					headers[k] = fmt.Sprintf("%v", val)
				}
			}
			if b, ok := options["body"].(string); ok {
				body = b
			} else if b, ok := options["body"].(map[string]interface{}); ok {
				bodyBytes, _ := json.Marshal(b)
				body = string(bodyBytes)
			}
		}
		callback, ok = goja.AssertFunction(call.Arguments[2])
		if !ok {
			panic(r.vm.NewGoError(fmt.Errorf("http.%s 最后一个参数必须是回调函数", strings.ToLower(method))))
		}
	}

	if reqURL == "" {
		panic(r.vm.NewGoError(fmt.Errorf("http.%s 需要有效的 URL", strings.ToLower(method))))
	}

	// 同步执行 HTTP 请求（goja 不支持真正的异步）
	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequest(method, reqURL, strings.NewReader(body))
	} else {
		req, err = http.NewRequest(method, reqURL, nil)
	}

	if err != nil {
		callback(goja.Undefined(), r.vm.ToValue(err.Error()), goja.Null())
		return goja.Undefined()
	}

	// 设置默认 Content-Type
	if body != "" && headers["Content-Type"] == "" {
		if headers == nil {
			headers = make(map[string]string)
		}
		headers["Content-Type"] = "application/json"
	}

	// 设置请求头
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		callback(goja.Undefined(), r.vm.ToValue(err.Error()), goja.Null())
		return goja.Undefined()
	}
	defer resp.Body.Close()

	// 读取响应体
	bodyBytes := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			bodyBytes = append(bodyBytes, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	bodyStr := string(bodyBytes)

	// 构建响应对象
	respObj := r.vm.NewObject()
	respObj.Set("code", resp.StatusCode)
	respObj.Set("status", resp.Status)
	respObj.Set("text", bodyStr)

	// 解析响应头
	respHeaders := r.vm.NewObject()
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders.Set(k, v[0])
		}
	}
	respObj.Set("headers", respHeaders)

	// 尝试解析 JSON
	var jsonBody interface{}
	if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
		respObj.Set("body", r.vm.ToValue(jsonBody))
	} else {
		respObj.Set("body", bodyStr)
	}

	// json() 方法
	respObj.Set("json", func(call goja.FunctionCall) goja.Value {
		var result interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			panic(r.vm.NewGoError(fmt.Errorf("JSON 解析错误: %v", err)))
		}
		return r.vm.ToValue(result)
	})

	// 调用回调
	callback(goja.Undefined(), goja.Null(), respObj)
	return goja.Undefined()
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
