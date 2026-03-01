package ai

import "yqhp/workflow-engine/internal/executor"

func init() {
	unified := NewUnifiedAgentExecutor()
	executor.MustRegister(unified)

	// 旧类型别名映射，确保向后兼容
	executor.RegisterAlias("ai", UnifiedAgentType)
	executor.RegisterAlias("ai_chat", UnifiedAgentType)
	executor.RegisterAlias("ai_plan_execute", UnifiedAgentType)
	executor.RegisterAlias("ai_reflection", UnifiedAgentType)
	executor.RegisterAlias("ai_supervisor", UnifiedAgentType)
	executor.RegisterAlias("ai_deep_agent", UnifiedAgentType)
}
