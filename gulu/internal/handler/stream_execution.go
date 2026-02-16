package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"yqhp/common/response"
	"yqhp/gulu/internal/executor"
	"yqhp/gulu/internal/logic"
	"yqhp/gulu/internal/middleware"
	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/scheduler"
	"yqhp/gulu/internal/sse"
	"yqhp/gulu/internal/workflow"
	"yqhp/workflow-engine/pkg/logger"
	"yqhp/workflow-engine/pkg/types"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

// StreamExecutionHandler 流式执行处理器
type StreamExecutionHandler struct {
	scheduler      scheduler.Scheduler
	streamExecutor *executor.StreamExecutor
	sessionManager *executor.SessionManager
}

// NewStreamExecutionHandler 创建流式执行处理器
func NewStreamExecutionHandler(sched scheduler.Scheduler, streamExec *executor.StreamExecutor, sessionMgr *executor.SessionManager) *StreamExecutionHandler {
	return &StreamExecutionHandler{
		scheduler:      sched,
		streamExecutor: streamExec,
		sessionManager: sessionMgr,
	}
}

// ExecuteRequest 统一执行请求
type ExecuteRequest struct {
	// 工作流定义（完整工作流）
	Workflow interface{} `json:"workflow,omitempty"`
	// 单步快捷方式：传入单个步骤，自动包装为工作流
	Step *StepConfig `json:"step,omitempty"`
	// 环境 ID
	EnvID int64 `json:"envId,omitempty"`
	// 变量
	Variables map[string]interface{} `json:"variables,omitempty"`
	// 执行模式：debug（失败即停止）或 normal（继续执行）
	Mode string `json:"mode,omitempty"`
	// 会话 ID
	SessionID string `json:"sessionId,omitempty"`
	// 选中的步骤 ID
	SelectedSteps []string `json:"selectedSteps,omitempty"`
	// 超时时间（秒）
	Timeout int `json:"timeout,omitempty"`
	// 执行器类型
	ExecutorType string `json:"executorType,omitempty"`
	// 指定的 Slave ID
	SlaveID string `json:"slaveId,omitempty"`
	// 是否使用 SSE 流式响应
	Stream bool `json:"stream,omitempty"`
	// 是否持久化执行记录
	Persist *bool `json:"persist,omitempty"`
}

// StepConfig 步骤配置（单步执行快捷方式）
type StepConfig struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	Config         map[string]interface{} `json:"config"`
	PreProcessors  []ProcessorConfig      `json:"preProcessors,omitempty"`
	PostProcessors []ProcessorConfig      `json:"postProcessors,omitempty"`
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Name    string                 `json:"name,omitempty"`
	Config  map[string]interface{} `json:"config"`
}

// Execute 统一执行入口
// POST /api/execute
// 通过 stream 参数或 Accept 头判断响应方式：
// - stream=true 或 Accept: text/event-stream → SSE 流式响应
// - 否则 → 阻塞式 JSON 响应
func (h *StreamExecutionHandler) Execute(c *fiber.Ctx) error {
	var req ExecuteRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败: "+err.Error())
	}

	// 判断是否使用 SSE 流式响应
	isSSE := req.Stream || strings.Contains(c.Get("Accept"), "text/event-stream")

	// 处理 Step 快捷方式：将单个步骤包装为工作流
	var workflowDef interface{}
	if req.Step != nil {
		// 如果是 AI 节点且含 ai_model_id，预解析托管模型配置
		if req.Step.Type == "ai" {
			if err := h.resolveAIModelConfig(c, req.Step.Config); err != nil {
				return response.Error(c, "解析 AI 模型失败: "+err.Error())
			}
		}

		// 构建单步工作流
		workflowDef = map[string]interface{}{
			"steps": []interface{}{
				map[string]interface{}{
					"id":             req.Step.ID,
					"type":           req.Step.Type,
					"name":           req.Step.Name,
					"config":         req.Step.Config,
					"preProcessors":  req.Step.PreProcessors,
					"postProcessors": req.Step.PostProcessors,
				},
			},
		}
	} else if req.Workflow != nil {
		// 对完整工作流定义中的 AI 节点进行模型解析
		workflowDef = req.Workflow
		h.resolveAIModelConfigsInWorkflow(c, workflowDef)
	} else {
		return response.Error(c, "工作流定义不能为空（需要提供 workflow 或 step）")
	}

	// 流程执行时环境ID必填，单步调试时可选
	if req.Workflow != nil && req.EnvID <= 0 {
		return response.Error(c, "流程执行时环境ID不能为空")
	}

	// 默认调试模式
	mode := req.Mode
	if mode == "" {
		mode = "debug"
	}

	// 准备执行上下文（无 workflowID，直接使用定义）
	execCtx, err := h.prepareExecutionFromDefinition(c, workflowDef, req.EnvID, req.Variables, mode, shouldPersist(req.Persist), req.ExecutorType, req.SlaveID, req.Timeout, req.SessionID)
	if err != nil {
		if execErr, ok := err.(*executionError); ok {
			if execErr.code == "NOT_FOUND" {
				return response.NotFound(c, execErr.message)
			}
		}
		return response.Error(c, err.Error())
	}

	// 过滤选中的步骤
	if len(req.SelectedSteps) > 0 {
		logger.Debug("过滤选中的步骤", "count", len(req.SelectedSteps))
		execCtx.EngineWf.Steps = filterSteps(execCtx.EngineWf.Steps, req.SelectedSteps)
	}

	if isSSE {
		return h.executeSSE(c, execCtx)
	}
	return h.executeBlocking(c, execCtx)
}

// prepareExecutionFromDefinition 从工作流定义准备执行上下文
func (h *StreamExecutionHandler) prepareExecutionFromDefinition(c *fiber.Ctx, definition interface{}, envID int64, variables map[string]interface{}, mode string, persist bool, executorType string, slaveID string, timeout int, sessionID string) (*ExecutionContext, error) {
	userID := middleware.GetCurrentUserID(c)

	// 生成会话 ID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// 确定执行模式
	execMode := scheduler.ModeDebug
	if mode == "normal" {
		execMode = scheduler.ModeExecute
	}

	// 转换工作流定义
	var definitionStr string
	switch v := definition.(type) {
	case string:
		definitionStr = v
	case map[string]interface{}:
		defBytes, err := json.Marshal(v)
		if err != nil {
			return nil, &executionError{code: "PARSE_ERROR", message: "工作流定义序列化失败: " + err.Error()}
		}
		definitionStr = string(defBytes)
	default:
		defBytes, err := json.Marshal(v)
		if err != nil {
			return nil, &executionError{code: "PARSE_ERROR", message: "工作流定义序列化失败: " + err.Error()}
		}
		definitionStr = string(defBytes)
	}

	// 环境变量快照（用于调试上下文中区分环境变量和临时变量）
	var envVarsSnapshot map[string]interface{}
	var mergedConfig *workflow.MergedConfig

	// 如果有环境ID，加载环境配置（包含环境变量、域名、数据库、MQ等）
	if envID > 0 {
		merger := workflow.NewConfigMerger(c.UserContext(), envID)
		mc, err := merger.Merge()
		if err != nil {
			logger.Warn("加载环境配置失败", "envId", envID, "error", err)
			// 不阻断执行，只记录警告
		} else {
			mergedConfig = mc

			// 保存环境变量快照
			envVarsSnapshot = make(map[string]interface{}, len(mergedConfig.Variables))
			for k, v := range mergedConfig.Variables {
				envVarsSnapshot[k] = v
			}

			// 将环境变量合并到请求变量中（请求变量优先级高于环境变量）
			if variables == nil {
				variables = make(map[string]interface{})
			}
			for k, v := range mergedConfig.Variables {
				if _, exists := variables[k]; !exists {
					variables[k] = v
				}
			}
		}
	}

	// 转换工作流
	var engineWf *types.Workflow
	var err error
	if mode == "normal" {
		engineWf, err = logic.ConvertToEngineWorkflow(definitionStr, sessionID)
	} else {
		engineWf, err = logic.ConvertToEngineWorkflowStopOnError(definitionStr, sessionID)
	}
	if err != nil {
		return nil, &executionError{code: "CONVERT_ERROR", message: "工作流转换失败: " + err.Error()}
	}

	logger.Debug("工作流转换完成", "id", engineWf.ID, "name", engineWf.Name, "steps", len(engineWf.Steps))

	// 解析步骤中的环境配置引用（域名、数据库、MQ）
	// 将 domainCode/database_config/mq_config 等引用解析为执行器能直接消费的实际配置
	if mergedConfig != nil {
		resolveEnvConfigReferences(engineWf.Steps, mergedConfig)
	}

	// 保存环境变量快照到工作流（用于调试上下文区分环境变量和临时变量）
	if envVarsSnapshot != nil {
		engineWf.EnvVariables = envVarsSnapshot
	}

	// 创建执行记录（仅当 persist=true 时）
	var execLogic *logic.ExecutionLogic
	if persist {
		execLogic = logic.NewExecutionLogic(c.UserContext())
		modeStr := string(model.ExecutionModeDebug)
		if mode == "normal" {
			modeStr = string(model.ExecutionModeExecute)
		}
		// 无 workflowID 时使用 0
		if err := execLogic.CreateStreamExecution(sessionID, 0, 0, envID, userID, modeStr); err != nil {
			return nil, &executionError{code: "DB_ERROR", message: "创建执行记录失败: " + err.Error()}
		}
	}

	// 创建执行请求
	execReq := &executor.ExecuteRequest{
		WorkflowID:   0,
		EnvID:        envID,
		Variables:    variables,
		Timeout:      timeout,
		ExecutorType: executor.ExecutorType(executorType),
		SlaveID:      slaveID,
	}

	return &ExecutionContext{
		WorkflowID:  0,
		SessionID:   sessionID,
		EnvID:       envID,
		UserID:      userID,
		EngineWf:    engineWf,
		Persist:     persist,
		ExecMode:    execMode,
		ExecLogic:   execLogic,
		ScheduleRes: nil,
		ExecReq:     execReq,
	}, nil
}

// executeSSE SSE 流式执行
func (h *StreamExecutionHandler) executeSSE(c *fiber.Ctx, execCtx *ExecutionContext) error {
	// 设置 SSE 响应头
	c.Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("X-Accel-Buffering", "no")
	c.Set("X-Content-Type-Options", "nosniff")

	// 捕获上下文
	ctx := c.UserContext()

	// 使用 StreamWriter 处理 SSE
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("SSE StreamWriter panic", "error", r)
			}
		}()

		writer := sse.NewWriter(&flushWriter{w: w}, execCtx.SessionID)

		// 发送连接成功事件
		writer.WriteEvent(&sse.Event{
			Type: "connected",
			Data: map[string]interface{}{
				"sessionId": execCtx.SessionID,
				"message":   "SSE 连接成功",
				"persist":   execCtx.Persist,
			},
		})

		// 执行工作流
		execErr := h.streamExecutor.ExecuteStream(ctx, execCtx.ExecReq, execCtx.EngineWf, writer)

		// 更新执行记录状态
		h.updateExecutionStatus(execCtx, execErr, writer)
	})

	return nil
}

// executeBlocking 阻塞式执行
func (h *StreamExecutionHandler) executeBlocking(c *fiber.Ctx, execCtx *ExecutionContext) error {
	// 执行工作流（阻塞）
	// ExecuteBlocking 保证：执行层面的失败（HTTP 错误、断言失败等）体现在 summary 中，不返回 error
	// 仅基础设施层面的失败（会话创建失败等）才返回 error
	summary, execErr := h.streamExecutor.ExecuteBlocking(c.UserContext(), execCtx.ExecReq, execCtx.EngineWf)
	if execErr != nil {
		// 基础设施错误，无法产生执行结果
		if execCtx.Persist && execCtx.ExecLogic != nil {
			execCtx.ExecLogic.UpdateStreamExecutionStatus(execCtx.SessionID, string(model.ExecutionStatusFailed), nil)
		}
		return response.Error(c, "执行失败: "+execErr.Error())
	}

	// 更新执行记录状态
	if execCtx.Persist && execCtx.ExecLogic != nil {
		execCtx.ExecLogic.UpdateStreamExecutionStatus(execCtx.SessionID, summary.Status, summary)
	}

	return response.Success(c, summary)
}

// InteractionRequest 交互请求
type InteractionRequest struct {
	Value   string `json:"value"`
	Skipped bool   `json:"skipped"`
}

// ExecutionContext 执行上下文（公共逻辑抽取）
type ExecutionContext struct {
	WorkflowID   int64
	SessionID    string
	EnvID        int64
	UserID       int64
	EngineWf     *types.Workflow
	Persist      bool
	ExecMode     scheduler.ExecutionMode
	ExecLogic    *logic.ExecutionLogic
	ScheduleRes  *scheduler.ScheduleResult
	ExecReq      *executor.ExecuteRequest
}

// shouldPersist 判断是否需要持久化
func shouldPersist(persist *bool) bool {
	if persist == nil {
		return true
	}
	return *persist
}


// executionError 执行错误
type executionError struct {
	code    string
	message string
}

func (e *executionError) Error() string {
	return e.message
}

// updateExecutionStatus 更新执行状态（SSE 模式）
func (h *StreamExecutionHandler) updateExecutionStatus(execCtx *ExecutionContext, execErr error, writer *sse.Writer) {
	if execCtx.Persist && execCtx.ExecLogic != nil {
		if execErr != nil {
			writer.WriteErrorCode(sse.ErrExecutorError, "执行失败", execErr.Error())
			execCtx.ExecLogic.UpdateStreamExecutionStatus(execCtx.SessionID, string(model.ExecutionStatusFailed), nil)
		} else {
			session, ok := h.sessionManager.GetSession(execCtx.SessionID)
			if ok {
				total, success, failed := session.GetStats()
				status := "success"
				if failed > 0 {
					status = "failed"
				}
				execCtx.ExecLogic.UpdateStreamExecutionStatus(execCtx.SessionID, status, map[string]interface{}{
					"totalSteps":   total,
					"successSteps": success,
					"failedSteps":  failed,
				})
			}
		}
	} else if execErr != nil {
		writer.WriteErrorCode(sse.ErrExecutorError, "执行失败", execErr.Error())
	}
}

// StopExecution 停止执行
// DELETE /api/executions/:sessionId/stop
func (h *StreamExecutionHandler) StopExecution(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return response.Error(c, "会话ID不能为空")
	}

	if err := h.streamExecutor.Stop(sessionID); err != nil {
		return response.Error(c, "停止执行失败: "+err.Error())
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())
	executionLogic.UpdateStreamExecutionStatus(sessionID, string(model.ExecutionStatusStopped), nil)

	return response.Success(c, nil)
}

// SubmitInteraction 提交交互响应
// POST /api/executions/:sessionId/interaction
func (h *StreamExecutionHandler) SubmitInteraction(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return response.Error(c, "会话ID不能为空")
	}

	var req InteractionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败: "+err.Error())
	}

	resp := &executor.InteractionResponse{
		Value:   req.Value,
		Skipped: req.Skipped,
	}

	if err := h.sessionManager.SubmitInteraction(sessionID, resp); err != nil {
		return response.Error(c, "提交交互响应失败: "+err.Error())
	}

	return response.Success(c, nil)
}

// GetExecutionStatus 获取执行状态
// GET /api/executions/:sessionId/status
func (h *StreamExecutionHandler) GetExecutionStatus(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return response.Error(c, "会话ID不能为空")
	}

	// 先检查内存中的会话
	session, ok := h.sessionManager.GetSession(sessionID)
	if ok {
		total, success, failed := session.GetStats()
		return response.Success(c, map[string]interface{}{
			"sessionId":    sessionID,
			"status":       session.GetStatus(),
			"totalSteps":   total,
			"successSteps": success,
			"failedSteps":  failed,
			"startTime":    session.StartTime,
			"durationMs":   time.Since(session.StartTime).Milliseconds(),
		})
	}

	// 从数据库获取
	executionLogic := logic.NewExecutionLogic(c.UserContext())
	dbSession, err := executionLogic.GetStreamExecution(sessionID)
	if err != nil {
		return response.NotFound(c, "执行记录不存在")
	}

	return response.Success(c, dbSession)
}

// flushWriter 包装 bufio.Writer 以支持 Flush
type flushWriter struct {
	w *bufio.Writer
}

func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if err != nil {
		return
	}
	err = fw.w.Flush()
	return
}

func (fw *flushWriter) Flush() {
	fw.w.Flush()
}

// fasthttpFlushWriter 用于 fasthttp 的 flush writer
type fasthttpFlushWriter struct {
	ctx *fasthttp.RequestCtx
}

func (fw *fasthttpFlushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.ctx.Write(p)
	if err != nil {
		return
	}
	fw.ctx.Response.ImmediateHeaderFlush = true
	return
}

func (fw *fasthttpFlushWriter) Flush() {}

// resolveAIModelConfig 解析 AI 节点中的托管模型配置
// 如果 config 中包含 ai_model_id，则从数据库获取模型的完整配置（api_key、base_url、model、provider）并注入到 config 中
func (h *StreamExecutionHandler) resolveAIModelConfig(c *fiber.Ctx, config map[string]interface{}) error {
	aiModelIDRaw, ok := config["ai_model_id"]
	if !ok || aiModelIDRaw == nil {
		return nil
	}

	// 提取 ai_model_id（可能是 float64 或 int）
	var aiModelID int64
	switch v := aiModelIDRaw.(type) {
	case float64:
		aiModelID = int64(v)
	case int:
		aiModelID = int64(v)
	case int64:
		aiModelID = v
	default:
		return nil
	}

	if aiModelID <= 0 {
		return nil
	}

	// 从数据库获取完整模型信息（含 API Key）
	aiModelLogic := logic.NewAiModelLogic(c.Context())
	aiModel, err := aiModelLogic.GetByIDWithKey(aiModelID)
	if err != nil {
		return fmt.Errorf("AI 模型不存在或已删除 (ID=%d): %v", aiModelID, err)
	}

	// 检查模型状态
	if aiModel.Status != nil && *aiModel.Status != 1 {
		return fmt.Errorf("AI 模型已禁用 (ID=%d, Name=%s)", aiModelID, aiModel.Name)
	}

	// 注入模型配置到 step config
	config["provider"] = aiModel.Provider
	config["model"] = aiModel.ModelID
	config["api_key"] = aiModel.APIKey
	config["base_url"] = aiModel.APIBaseURL

	return nil
}

// resolveAIModelConfigsInWorkflow 递归解析工作流定义中所有 AI 节点的托管模型配置
func (h *StreamExecutionHandler) resolveAIModelConfigsInWorkflow(c *fiber.Ctx, workflowDef interface{}) {
	wfMap, ok := workflowDef.(map[string]interface{})
	if !ok {
		return
	}

	stepsRaw, ok := wfMap["steps"]
	if !ok {
		return
	}

	steps, ok := stepsRaw.([]interface{})
	if !ok {
		return
	}

	for _, stepRaw := range steps {
		stepMap, ok := stepRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// 如果是 AI 类型的步骤，解析模型配置
		if stepType, _ := stepMap["type"].(string); stepType == "ai" {
			if configRaw, ok := stepMap["config"]; ok {
				if config, ok := configRaw.(map[string]interface{}); ok {
					if err := h.resolveAIModelConfig(c, config); err != nil {
						logger.Warn("解析 AI 模型配置失败", "error", err)
					}
				}
			}
		}

		// 递归处理子步骤（循环、条件等）
		if children, ok := stepMap["children"].([]interface{}); ok {
			for _, child := range children {
				if childMap, ok := child.(map[string]interface{}); ok {
					h.resolveAIModelConfigsInWorkflow(c, childMap)
				}
			}
		}
		if loopRaw, ok := stepMap["loop"].(map[string]interface{}); ok {
			if loopSteps, ok := loopRaw["steps"].([]interface{}); ok {
				h.resolveAIModelConfigsInWorkflow(c, map[string]interface{}{"steps": loopSteps})
			}
		}
		if branches, ok := stepMap["branches"].([]interface{}); ok {
			for _, branch := range branches {
				if branchMap, ok := branch.(map[string]interface{}); ok {
					if branchSteps, ok := branchMap["steps"].([]interface{}); ok {
						h.resolveAIModelConfigsInWorkflow(c, map[string]interface{}{"steps": branchSteps})
					}
				}
			}
		}
	}
}

// filterSteps 过滤选中的步骤
func filterSteps(steps []types.Step, selectedIDs []string) []types.Step {
	if len(selectedIDs) == 0 {
		return steps
	}

	idSet := make(map[string]bool)
	for _, id := range selectedIDs {
		idSet[id] = true
	}

	var filtered []types.Step
	for _, step := range steps {
		if idSet[step.ID] {
			filtered = append(filtered, step)
		} else {
			// 对于循环步骤，递归过滤子步骤
			if step.Loop != nil && len(step.Loop.Steps) > 0 {
				filteredChildren := filterSteps(step.Loop.Steps, selectedIDs)
				if len(filteredChildren) > 0 {
					newStep := step
					newStep.Loop = &types.Loop{
						Mode:              step.Loop.Mode,
						Count:             step.Loop.Count,
						Items:             step.Loop.Items,
						ItemVar:           step.Loop.ItemVar,
						Condition:         step.Loop.Condition,
						MaxIterations:     step.Loop.MaxIterations,
						BreakCondition:    step.Loop.BreakCondition,
						ContinueCondition: step.Loop.ContinueCondition,
						Steps:             filteredChildren,
					}
					filtered = append(filtered, newStep)
				}
			}
			// 对于条件步骤，递归过滤 children
			if len(step.Children) > 0 {
				filteredChildren := filterSteps(step.Children, selectedIDs)
				if len(filteredChildren) > 0 {
					newStep := step
					newStep.Children = filteredChildren
					filtered = append(filtered, newStep)
				}
			}
		}
	}

	return filtered
}

// resolveEnvConfigReferences 解析步骤中的环境配置引用。
// 将前端传入的配置引用（domainCode、database_config、mq_config）解析为
// 执行器能直接消费的实际配置值，注入到步骤 config 中。
func resolveEnvConfigReferences(steps []types.Step, config *workflow.MergedConfig) {
	for i := range steps {
		step := &steps[i]

		switch step.Type {
		case "http":
			resolveHTTPDomainConfig(step.Config, config)
		case "db", "database":
			resolveDatabaseConfig(step.Config, config)
		case "mq":
			resolveMQConfig(step.Config, config)
		}

		// 递归处理嵌套步骤（条件分支、循环等）
		if step.Loop != nil && len(step.Loop.Steps) > 0 {
			resolveEnvConfigReferences(step.Loop.Steps, config)
		}
		if len(step.Children) > 0 {
			resolveEnvConfigReferences(step.Children, config)
		}
		for bi := range step.Branches {
			if len(step.Branches[bi].Steps) > 0 {
				resolveEnvConfigReferences(step.Branches[bi].Steps, config)
			}
		}
	}
}

// resolveHTTPDomainConfig 解析 HTTP 步骤的域名配置引用。
// 将 domainCode 解析为 domain_base_url 和 domain_headers，
// 执行器可直接用于 URL 拼接和请求头注入。
func resolveHTTPDomainConfig(stepConfig map[string]interface{}, config *workflow.MergedConfig) {
	if stepConfig == nil || config.Domains == nil {
		return
	}

	// 获取域名引用（前端字段名为 domainCode）
	domainCode, _ := stepConfig["domainCode"].(string)
	if domainCode == "" {
		domainCode, _ = stepConfig["domain"].(string)
	}
	if domainCode == "" {
		return
	}

	dc, ok := config.Domains[domainCode]
	if !ok || dc == nil {
		return
	}

	// 注入解析后的域名配置
	stepConfig["domain"] = domainCode
	stepConfig["domain_base_url"] = dc.BaseURL
	if len(dc.Headers) > 0 {
		stepConfig["domain_headers"] = dc.Headers
	}
	// 清理前端字段
	delete(stepConfig, "domainCode")
}

// resolveDatabaseConfig 解析数据库步骤的配置引用。
// 将 database_config / datasourceCode（引用 code）展开为 driver、dsn 等执行器可消费的字段。
// 同时规范化前端字段名与执行器期望的字段名之间的差异。
func resolveDatabaseConfig(stepConfig map[string]interface{}, config *workflow.MergedConfig) {
	if stepConfig == nil {
		return
	}

	// === 1. 规范化前端参数 ===

	// 前端 params 是 [{key, value, ...}] 对象数组，执行器期望 [value1, value2, ...]
	if params, ok := stepConfig["params"].([]interface{}); ok && len(params) > 0 {
		plainParams := make([]interface{}, 0, len(params))
		for _, p := range params {
			if pMap, ok := p.(map[string]interface{}); ok {
				if v, exists := pMap["value"]; exists {
					plainParams = append(plainParams, v)
				}
			} else {
				plainParams = append(plainParams, p)
			}
		}
		stepConfig["params"] = plainParams
	}

	// 处理 settings 中的 timeout（毫秒 -> 持续时间字符串）
	if settings, ok := stepConfig["settings"].(map[string]interface{}); ok {
		if timeout, ok := settings["timeout"].(float64); ok && timeout > 0 {
			stepConfig["timeout"] = fmt.Sprintf("%dms", int(timeout))
		}
	}

	// 旧格式兼容：旧版使用 "query" 字段存储 SQL
	if _, hasSQL := stepConfig["sql"]; !hasSQL {
		if query, ok := stepConfig["query"].(string); ok {
			stepConfig["sql"] = query
		}
	}

	// 旧格式兼容：如果没有 action，默认为 query
	if _, hasAction := stepConfig["action"]; !hasAction {
		stepConfig["action"] = "query"
	}

	// === 2. 解析数据源配置引用 ===

	if config == nil || config.Databases == nil {
		return
	}

	// 兼容新旧字段名：新前端使用 datasourceCode，旧配置使用 database_config
	dbCode, _ := stepConfig["database_config"].(string)
	if dbCode == "" {
		dbCode, _ = stepConfig["datasourceCode"].(string)
	}
	if dbCode == "" {
		return
	}

	dc, ok := config.Databases[dbCode]
	if !ok || dc == nil {
		return
	}

	// 注入数据库驱动
	stepConfig["driver"] = dc.Type

	// 构建 DSN
	dsn := buildDSN(dc)
	if dsn != "" {
		stepConfig["dsn"] = dsn
	}
}

// resolveMQConfig 解析 MQ 步骤的配置引用。
// 将 mq_config（引用 code）展开为 type、broker、auth 等执行器可消费的字段。
func resolveMQConfig(stepConfig map[string]interface{}, config *workflow.MergedConfig) {
	if stepConfig == nil || config.MQs == nil {
		return
	}

	mqCode, _ := stepConfig["mq_config"].(string)
	if mqCode == "" {
		return
	}

	mc, ok := config.MQs[mqCode]
	if !ok || mc == nil {
		return
	}

	// 注入 MQ 配置
	stepConfig["type"] = mc.Type
	stepConfig["broker"] = fmt.Sprintf("%s:%d", mc.Host, mc.Port)

	if mc.Username != "" || mc.Password != "" {
		stepConfig["auth"] = map[string]interface{}{
			"username": mc.Username,
			"password": mc.Password,
		}
	}
	if mc.VHost != "" {
		if opts, ok := stepConfig["options"].(map[string]interface{}); ok {
			opts["vhost"] = mc.VHost
		} else {
			stepConfig["options"] = map[string]interface{}{"vhost": mc.VHost}
		}
	}
}

// buildDSN 根据数据库配置构建 DSN 连接字符串。
func buildDSN(dc *workflow.DatabaseConfig) string {
	switch strings.ToLower(dc.Type) {
	case "mysql":
		// user:password@tcp(host:port)/database?options
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			dc.Username, dc.Password, dc.Host, dc.Port, dc.Database)
		if dc.Options != "" {
			dsn += "?" + dc.Options
		} else {
			dsn += "?charset=utf8mb4&parseTime=True&loc=Local"
		}
		return dsn
	case "postgres", "postgresql":
		// host=X port=X user=X password=X dbname=X sslmode=disable
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
			dc.Host, dc.Port, dc.Username, dc.Password, dc.Database)
		if dc.Options != "" {
			dsn += " " + dc.Options
		} else {
			dsn += " sslmode=disable"
		}
		return dsn
	case "redis":
		// addr:port
		if dc.Password != "" {
			return fmt.Sprintf("redis://%s:%s@%s:%d", dc.Username, dc.Password, dc.Host, dc.Port)
		}
		return fmt.Sprintf("%s:%d", dc.Host, dc.Port)
	case "mongodb":
		// mongodb://user:password@host:port/database
		if dc.Username != "" {
			return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s",
				dc.Username, dc.Password, dc.Host, dc.Port, dc.Database)
		}
		return fmt.Sprintf("mongodb://%s:%d/%s", dc.Host, dc.Port, dc.Database)
	default:
		// 通用格式
		return fmt.Sprintf("%s:%s@%s:%d/%s",
			dc.Username, dc.Password, dc.Host, dc.Port, dc.Database)
	}
}
