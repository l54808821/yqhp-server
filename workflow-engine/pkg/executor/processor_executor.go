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

// ProcessorExecutor 处理器执行器
type ProcessorExecutor struct {
	variables map[string]interface{} // 变量上下文
	envVars   map[string]interface{} // 环境变量
	response  map[string]interface{} // HTTP 响应数据
}

// NewProcessorExecutor 创建处理器执行器
func NewProcessorExecutor(variables, envVars map[string]interface{}) *ProcessorExecutor {
	return &ProcessorExecutor{
		variables: variables,
		envVars:   envVars,
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

	// 准备运行时配置
	rtConfig := &script.JSRuntimeConfig{
		Variables: make(map[string]interface{}),
		EnvVars:   make(map[string]interface{}),
	}

	// 注入变量
	for k, v := range e.variables {
		rtConfig.Variables[k] = v
	}
	for k, v := range e.envVars {
		rtConfig.EnvVars[k] = v
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

	// 更新变量
	for k, v := range execResult.Variables {
		e.variables[k] = v
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
	if name, ok := pctx.processor.Config["variableName"].(string); ok {
		varName = name
	}
	if value, ok := pctx.processor.Config["value"].(string); ok {
		varValue = e.replaceVariables(value)
	}

	if varName != "" {
		e.variables[varName] = varValue
		pctx.message = fmt.Sprintf("%s = %s", varName, varValue)
		pctx.output = map[string]any{
			"variableName": varName,
			"value":        varValue,
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

	if et, ok := pctx.processor.Config["extractType"].(string); ok {
		extractType = et
	}
	if exp, ok := pctx.processor.Config["expression"].(string); ok {
		expression = exp
	}
	if name, ok := pctx.processor.Config["variableName"].(string); ok {
		varName = name
	}

	value, err := e.extractValue(extractType, expression)
	if err != nil {
		pctx.success = false
		pctx.message = err.Error()
		return
	}

	if varName != "" {
		e.variables[varName] = value
		pctx.message = fmt.Sprintf("%s = %v", varName, value)
		pctx.output = map[string]any{
			"variableName": varName,
			"value":        value,
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
func (e *ProcessorExecutor) replaceVariables(s string) string {
	result := s
	for k, v := range e.variables {
		placeholder := "${" + k + "}"
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
	}
	for k, v := range e.envVars {
		placeholder := "${" + k + "}"
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
	}
	return result
}

// GetVariables 获取更新后的变量
func (e *ProcessorExecutor) GetVariables() map[string]interface{} {
	return e.variables
}
