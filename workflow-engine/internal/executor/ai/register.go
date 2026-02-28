package ai

import "yqhp/workflow-engine/internal/executor"

func init() {
	// 注册旧版 AI 执行器（保留兼容）
	executor.MustRegister(NewAIExecutor())

	// 注册 6 种新 AI 节点执行器
	executor.MustRegister(NewChatExecutor())
	executor.MustRegister(NewAgentExecutor())
	executor.MustRegister(NewPlanExecuteExecutor())
	executor.MustRegister(NewReflectionExecutor())
	executor.MustRegister(NewSupervisorExecutor())
	executor.MustRegister(NewDeepAgentExecutor())
}
