// Package executor provides public execution utilities for the workflow engine.
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"yqhp/workflow-engine/pkg/script"
	"yqhp/workflow-engine/pkg/types"
)

// VariableGetFunc 变量获取回调
type VariableGetFunc func(key string) (interface{}, bool)

// VariableSetFunc 变量设置回调（key, value, scope, source）
type VariableSetFunc func(key string, value interface{}, scope, source string)

// ProcessorExecutor 处理器执行器
// 通过回调委托变量操作给 ExecutionContext，实现实时同步。
type ProcessorExecutor struct {
	variables    map[string]interface{} // 本地变量缓存（用于 replaceVariables 等遍历场景）
	response     map[string]interface{} // HTTP 响应数据
	onGetVar     VariableGetFunc        // 变量获取回调
	onSetVar     VariableSetFunc        // 变量设置回调（带追踪）
}

// NewProcessorExecutor 创建处理器执行器
func NewProcessorExecutor(variables map[string]interface{}) *ProcessorExecutor {
	return &ProcessorExecutor{
		variables: variables,
	}
}

// NewProcessorExecutorWithCallbacks 创建带回调的处理器执行器
func NewProcessorExecutorWithCallbacks(variables map[string]interface{}, getVar VariableGetFunc, setVar VariableSetFunc) *ProcessorExecutor {
	return &ProcessorExecutor{
		variables: variables,
		onGetVar:  getVar,
		onSetVar:  setVar,
	}
}

// SetResponse 设置 HTTP 响应数据（用于后置处理器）
func (e *ProcessorExecutor) SetResponse(resp map[string]interface{}) {
	e.response = resp
}

// ExecuteProcessors 执行处理器列表
// phase: "pre" 表示前置处理器，"post" 表示后置处理器
func (e *ProcessorExecutor) ExecuteProcessors(ctx context.Context, processors []types.Processor, phase string) []types.ConsoleLogEntry {
	entries := make([]types.ConsoleLogEntry, 0)

	for _, processor := range processors {
		// 跳过禁用的处理器
		if !processor.Enabled {
			continue
		}

		// 执行处理器并收集日志
		procEntries := e.executeProcessor(ctx, processor, phase)
		entries = append(entries, procEntries...)
	}

	return entries
}

// processorContext 处理器执行上下文
type processorContext struct {
	processor types.Processor
	phase     string
	success   bool
	message   string
	output    map[string]any
	logs      []types.ConsoleLogEntry // 收集的日志条目
}

// executeProcessor 执行单个处理器，返回日志条目列表
func (e *ProcessorExecutor) executeProcessor(ctx context.Context, processor types.Processor, phase string) []types.ConsoleLogEntry {
	pctx := &processorContext{
		processor: processor,
		phase:     phase,
		success:   true,
		logs:      make([]types.ConsoleLogEntry, 0),
	}

	switch processor.Type {
	case "js_script":
		e.executeJsScript(ctx, pctx)

	case "set_variable":
		e.executeSetVariable(pctx)

	case "wait":
		e.executeWait(pctx)

	case "assertion":
		e.executeAssertion(pctx)

	case "extract_param":
		e.executeExtractParam(pctx)

	default:
		pctx.message = fmt.Sprintf("暂不支持的处理器类型: %s", processor.Type)
	}

	// 创建处理器执行结果日志条目
	procEntry := types.NewProcessorEntry(phase, types.ProcessorLogInfo{
		ID:      processor.ID,
		Type:    processor.Type,
		Name:    processor.Name,
		Success: pctx.success,
		Message: pctx.message,
		Output:  pctx.output,
	})

	// 返回：处理器条目 + 脚本产生的日志
	result := []types.ConsoleLogEntry{procEntry}
	result = append(result, pctx.logs...)

	return result
}

// setVariable 设置变量，优先通过回调同步到上下文，同时更新本地缓存
func (e *ProcessorExecutor) setVariable(key string, value interface{}, scope, source string) {
	e.variables[key] = value
	if e.onSetVar != nil {
		e.onSetVar(key, value, scope, source)
	}
}

// getVariable 获取变量，优先通过回调获取，降级到本地缓存
func (e *ProcessorExecutor) getVariable(key string) (interface{}, bool) {
	if e.onGetVar != nil {
		return e.onGetVar(key)
	}
	val, ok := e.variables[key]
	return val, ok
}

// executeJsScript 执行 JS 脚本
func (e *ProcessorExecutor) executeJsScript(ctx context.Context, pctx *processorContext) {
	scriptCode := ""
	if s, ok := pctx.processor.Config["script"].(string); ok {
		scriptCode = s
	}

	if scriptCode == "" {
		pctx.message = "脚本内容为空"
		return
	}

	// 准备运行时配置，直接传递统一 variables（含 env. 前缀的环境变量）
	rtConfig := &script.JSRuntimeConfig{
		Variables: make(map[string]interface{}, len(e.variables)),
	}
	for k, v := range e.variables {
		rtConfig.Variables[k] = v
	}

	// JS Runtime 也使用回调模式
	if e.onGetVar != nil {
		rtConfig.OnGetVariable = func(key string) (interface{}, bool) {
			return e.getVariable(key)
		}
	}
	if e.onSetVar != nil {
		rtConfig.OnSetVariable = func(key string, value interface{}) {
			scope := "temp"
			if strings.HasPrefix(key, "env.") {
				scope = "env"
			}
			e.setVariable(key, value, scope, "js_script")
		}
		rtConfig.OnDelVariable = func(key string) {
			e.setVariable(key, nil, "temp", "js_script")
		}
	}

	// 如果有响应数据，注入到运行时
	if e.response != nil {
		rtConfig.Response = e.response
	}

	// 创建 JS 运行时
	runtime := script.NewJSRuntime(rtConfig)

	// 执行脚本
	execResult, err := runtime.Execute(scriptCode, 30*time.Second)

	// 将脚本的 console.log 输出转换为日志条目
	for _, log := range execResult.ConsoleLogs {
		pctx.logs = append(pctx.logs, types.NewLogEntry(log))
	}

	if err != nil {
		pctx.success = false
		pctx.message = err.Error()
		return
	}

	// 如果没有回调，需要手动同步变量到本地缓存
	if e.onSetVar == nil {
		for k, v := range execResult.Variables {
			e.variables[k] = v
		}
	}

	pctx.message = "脚本执行成功"
	pctx.output = map[string]any{
		"result":    execResult.Value,
		"variables": execResult.Variables,
	}
}

// executeSetVariable 设置变量
func (e *ProcessorExecutor) executeSetVariable(pctx *processorContext) {
	varName := ""
	varValue := ""
	scope := "temp" // 默认临时变量

	if name, ok := pctx.processor.Config["variableName"].(string); ok {
		varName = name
	}
	if value, ok := pctx.processor.Config["value"].(string); ok {
		varValue = e.replaceVariables(value)
	}
	if s, ok := pctx.processor.Config["scope"].(string); ok && s != "" {
		scope = s
	}

	if varName != "" {
		storeKey := varName
		if scope == "env" {
			storeKey = "env." + varName
		}
		oldValue, _ := e.getVariable(storeKey)
		e.setVariable(storeKey, varValue, scope, "set_variable")
		pctx.message = fmt.Sprintf("%s = %s", varName, varValue)
		pctx.output = map[string]any{
			"variableName": varName,
			"value":        varValue,
			"oldValue":     oldValue,
			"scope":        scope,
			"source":       "set_variable",
		}
	}
}

// executeWait 等待
func (e *ProcessorExecutor) executeWait(pctx *processorContext) {
	duration := 1000 // 默认 1000ms
	if d, ok := pctx.processor.Config["duration"].(float64); ok {
		duration = int(d)
	}
	time.Sleep(time.Duration(duration) * time.Millisecond)
	pctx.message = fmt.Sprintf("等待 %dms 完成", duration)
}

// executeAssertion 执行断言
func (e *ProcessorExecutor) executeAssertion(pctx *processorContext) {
	if e.response == nil {
		pctx.success = false
		pctx.message = "无响应数据，无法执行断言"
		return
	}

	assertType := ""
	operator := ""
	expression := ""
	expected := ""

	if at, ok := pctx.processor.Config["assertType"].(string); ok {
		assertType = at
	}
	if op, ok := pctx.processor.Config["operator"].(string); ok {
		operator = op
	}
	if exp, ok := pctx.processor.Config["expression"].(string); ok {
		expression = exp
	}
	if exp, ok := pctx.processor.Config["expected"].(string); ok {
		expected = e.replaceVariables(exp)
	}

	passed, msg := e.doAssertion(assertType, operator, expression, expected)
	pctx.success = passed
	pctx.message = msg
}

// doAssertion 执行断言逻辑
func (e *ProcessorExecutor) doAssertion(assertType, operator, expression, expected string) (bool, string) {
	var actual string

	switch assertType {
	case "status_code", "statusCode":
		if code, ok := e.response["statusCode"].(int); ok {
			actual = fmt.Sprintf("%d", code)
		}
	case "response_body", "responseBody":
		if body, ok := e.response["body"].(string); ok {
			actual = body
		}
	case "jsonpath":
		if bodyRaw, ok := e.response["body"].(string); ok {
			var data interface{}
			if err := json.Unmarshal([]byte(bodyRaw), &data); err == nil {
				key := expression
				if len(key) > 2 && key[:2] == "$." {
					key = key[2:]
				}
				if m, ok := data.(map[string]interface{}); ok {
					if v, ok := m[key]; ok {
						actual = fmt.Sprintf("%v", v)
					}
				}
			}
		}
	case "header":
		if headers, ok := e.response["headers"].(map[string]interface{}); ok {
			if v, ok := headers[expression]; ok {
				actual = fmt.Sprintf("%v", v)
			}
		}
	case "response_time":
		if duration, ok := e.response["duration"].(int64); ok {
			actual = fmt.Sprintf("%d", duration)
		}
	default:
		return false, fmt.Sprintf("不支持的断言类型: %s", assertType)
	}

	// 执行比较
	passed := false
	switch operator {
	case "eq":
		passed = actual == expected
	case "ne":
		passed = actual != expected
	case "contains":
		passed = strings.Contains(actual, expected)
	case "not_contains":
		passed = !strings.Contains(actual, expected)
	case "gt":
		var actualNum, expectedNum float64
		fmt.Sscanf(actual, "%f", &actualNum)
		fmt.Sscanf(expected, "%f", &expectedNum)
		passed = actualNum > expectedNum
	case "lt":
		var actualNum, expectedNum float64
		fmt.Sscanf(actual, "%f", &actualNum)
		fmt.Sscanf(expected, "%f", &expectedNum)
		passed = actualNum < expectedNum
	case "gte":
		var actualNum, expectedNum float64
		fmt.Sscanf(actual, "%f", &actualNum)
		fmt.Sscanf(expected, "%f", &expectedNum)
		passed = actualNum >= expectedNum
	case "lte":
		var actualNum, expectedNum float64
		fmt.Sscanf(actual, "%f", &actualNum)
		fmt.Sscanf(expected, "%f", &expectedNum)
		passed = actualNum <= expectedNum
	default:
		return false, fmt.Sprintf("不支持的操作符: %s", operator)
	}

	if passed {
		return true, fmt.Sprintf("断言通过: %s %s %s (实际值: %s)", assertType, operator, expected, actual)
	}
	return false, fmt.Sprintf("断言失败: 期望 %s %s %s，实际值: %s", assertType, operator, expected, actual)
}

// executeExtractParam 提取参数
func (e *ProcessorExecutor) executeExtractParam(pctx *processorContext) {
	if e.response == nil {
		pctx.success = false
		pctx.message = "无响应数据，无法提取参数"
		return
	}

	extractType := ""
	expression := ""
	varName := ""
	scope := "temp" // 默认临时变量

	if et, ok := pctx.processor.Config["extractType"].(string); ok {
		extractType = et
	}
	if exp, ok := pctx.processor.Config["expression"].(string); ok {
		expression = exp
	}
	if name, ok := pctx.processor.Config["variableName"].(string); ok {
		varName = name
	}
	if s, ok := pctx.processor.Config["scope"].(string); ok && s != "" {
		scope = s
	}

	value, err := e.extractValue(extractType, expression)
	if err != nil {
		pctx.success = false
		pctx.message = err.Error()
		return
	}

	if varName != "" {
		storeKey := varName
		if scope == "env" {
			storeKey = "env." + varName
		}
		oldValue, _ := e.getVariable(storeKey)
		e.setVariable(storeKey, value, scope, "extract_param")
		pctx.message = fmt.Sprintf("%s = %v", varName, value)
		pctx.output = map[string]any{
			"variableName": varName,
			"value":        value,
			"oldValue":     oldValue,
			"scope":        scope,
			"source":       "extract_param",
		}
	}
}

// extractValue 从响应中提取值
func (e *ProcessorExecutor) extractValue(extractType, expression string) (interface{}, error) {
	switch extractType {
	case "jsonpath":
		if bodyRaw, ok := e.response["body"].(string); ok {
			var data interface{}
			if err := json.Unmarshal([]byte(bodyRaw), &data); err != nil {
				return nil, fmt.Errorf("解析 JSON 失败: %s", err.Error())
			}
			key := expression
			if len(key) > 2 && key[:2] == "$." {
				key = key[2:]
			}
			if m, ok := data.(map[string]interface{}); ok {
				if v, ok := m[key]; ok {
					return v, nil
				}
			}
		}
		return nil, fmt.Errorf("未找到路径: %s", expression)

	case "header":
		if headers, ok := e.response["headers"].(map[string]interface{}); ok {
			if v, ok := headers[expression]; ok {
				return v, nil
			}
		}
		return nil, fmt.Errorf("未找到 Header: %s", expression)

	case "cookie":
		if cookies, ok := e.response["cookies"].(map[string]interface{}); ok {
			if v, ok := cookies[expression]; ok {
				return v, nil
			}
		}
		return nil, fmt.Errorf("未找到 Cookie: %s", expression)

	default:
		return nil, fmt.Errorf("不支持的提取类型: %s", extractType)
	}
}

// replaceVariables 替换变量
// 统一处理所有变量（含 env. 前缀的环境变量），一次遍历即可
func (e *ProcessorExecutor) replaceVariables(s string) string {
	result := s
	for k, v := range e.variables {
		placeholder := "${" + k + "}"
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
	}
	return result
}

// GetVariables 获取更新后的变量
func (e *ProcessorExecutor) GetVariables() map[string]interface{} {
	return e.variables
}
