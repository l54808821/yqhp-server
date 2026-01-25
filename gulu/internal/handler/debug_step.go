package handler

import (
	"yqhp/common/response"
	"yqhp/gulu/internal/client"
	"yqhp/gulu/internal/executor"
	"yqhp/gulu/internal/logic"
	"yqhp/workflow-engine/pkg/types"

	"github.com/gofiber/fiber/v2"
)

// DebugStepHandler 单步调试处理器
type DebugStepHandler struct {
	sessionManager *executor.SessionManager
	engineClient   *client.WorkflowEngineClient
}

// NewDebugStepHandler 创建单步调试处理器
func NewDebugStepHandler(sessionMgr *executor.SessionManager) *DebugStepHandler {
	return &DebugStepHandler{
		sessionManager: sessionMgr,
		engineClient:   client.NewWorkflowEngineClient(),
	}
}

// DebugStepRequest 单步调试请求（与前端一致）
type DebugStepRequest struct {
	NodeConfig *DebugNodeConfig       `json:"nodeConfig"`
	EnvID      int64                  `json:"envId,omitempty"`
	Variables  map[string]interface{} `json:"variables,omitempty"`
	SessionID  string                 `json:"sessionId,omitempty"`
}

// DebugNodeConfig 调试节点配置
type DebugNodeConfig struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	Config         map[string]interface{} `json:"config"`
	PreProcessors  []KeywordConfig        `json:"preProcessors,omitempty"`
	PostProcessors []KeywordConfig        `json:"postProcessors,omitempty"`
}

// KeywordConfig 关键字配置
type KeywordConfig struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Name    string                 `json:"name,omitempty"`
	Config  map[string]interface{} `json:"config"`
}

// DebugStepResponse 单步调试响应（统一格式）
type DebugStepResponse struct {
	Success          bool                     `json:"success"`
	Response         *types.HTTPResponseData  `json:"response,omitempty"`
	ScriptResult     *DebugScriptResult       `json:"scriptResult,omitempty"`
	AssertionResults []types.AssertionResult  `json:"assertionResults,omitempty"`
	ConsoleLogs      []types.ConsoleLogEntry  `json:"consoleLogs,omitempty"`
	ActualRequest    *types.ActualRequest     `json:"actualRequest,omitempty"`
	Error            string                   `json:"error,omitempty"`
}

// DebugScriptResult 脚本执行结果
type DebugScriptResult struct {
	Script      string                  `json:"script"`
	Language    string                  `json:"language"`
	Result      interface{}             `json:"result"`
	ConsoleLogs []types.ConsoleLogEntry `json:"consoleLogs"`
	Error       string                  `json:"error,omitempty"`
	Variables   map[string]interface{}  `json:"variables"`
	DurationMs  int64                   `json:"durationMs"`
}

// DebugStep 单步调试 HTTP/Script 节点
// POST /api/debug/step
func (h *DebugStepHandler) DebugStep(c *fiber.Ctx) error {
	var req DebugStepRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败: "+err.Error())
	}

	if req.NodeConfig == nil {
		return response.Error(c, "节点配置不能为空")
	}

	if req.NodeConfig.Type != "http" && req.NodeConfig.Type != "script" {
		return response.Error(c, "目前只支持 HTTP 和脚本节点的单步调试")
	}

	// 合并变量
	variables := make(map[string]interface{})
	envVars := make(map[string]interface{})

	// 1. 从会话中获取变量
	if req.SessionID != "" && h.sessionManager != nil {
		session, ok := h.sessionManager.GetSession(req.SessionID)
		if ok {
			for k, v := range session.GetVariables() {
				variables[k] = v
			}
		}
	}

	// 2. 获取环境变量
	if req.EnvID > 0 {
		varLogic := logic.NewVarLogic(c.UserContext())
		vars, err := varLogic.GetVarsByEnvID(req.EnvID)
		if err == nil {
			for _, v := range vars {
				if v.Value != nil {
					envVars[v.Key] = *v.Value
				}
			}
		}
	}

	// 3. 请求变量优先级最高
	for k, v := range req.Variables {
		variables[k] = v
	}

	// 构建 workflow-engine 请求
	engineReq := &client.DebugStepRequest{
		NodeConfig: &client.DebugNodeConfig{
			ID:             req.NodeConfig.ID,
			Type:           req.NodeConfig.Type,
			Name:           req.NodeConfig.Name,
			Config:         req.NodeConfig.Config,
			PreProcessors:  convertToClientKeywordConfigs(req.NodeConfig.PreProcessors),
			PostProcessors: convertToClientKeywordConfigs(req.NodeConfig.PostProcessors),
		},
		EnvID:     req.EnvID,
		Variables: variables,
		EnvVars:   envVars,
		SessionID: req.SessionID,
	}

	// 调用 workflow-engine API 执行单步调试
	engineResp, err := h.engineClient.DebugStep(c.Context(), engineReq)
	if err != nil {
		return response.Error(c, "执行失败: "+err.Error())
	}

	// 转换响应格式
	result := &DebugStepResponse{
		Success:          engineResp.Success,
		Response:         engineResp.Response,
		AssertionResults: engineResp.AssertionResults,
		ConsoleLogs:      engineResp.ConsoleLogs,
		ActualRequest:    engineResp.ActualRequest,
		Error:            engineResp.Error,
	}

	// 转换脚本结果
	if engineResp.ScriptResult != nil {
		result.ScriptResult = &DebugScriptResult{
			Script:      engineResp.ScriptResult.Script,
			Language:    engineResp.ScriptResult.Language,
			Result:      engineResp.ScriptResult.Result,
			ConsoleLogs: engineResp.ScriptResult.ConsoleLogs,
			Error:       engineResp.ScriptResult.Error,
			Variables:   engineResp.ScriptResult.Variables,
			DurationMs:  engineResp.ScriptResult.DurationMs,
		}
	}

	return response.Success(c, result)
}

// convertToClientKeywordConfigs 转换关键字配置
func convertToClientKeywordConfigs(configs []KeywordConfig) []client.KeywordConfig {
	result := make([]client.KeywordConfig, len(configs))
	for i, c := range configs {
		result[i] = client.KeywordConfig{
			ID:      c.ID,
			Type:    c.Type,
			Enabled: c.Enabled,
			Name:    c.Name,
			Config:  c.Config,
		}
	}
	return result
}
