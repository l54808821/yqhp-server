package ai

import "yqhp/workflow-engine/internal/executor"

func init() {
	executor.MustRegister(NewUnifiedAgentExecutor())  // ai_agent（向后兼容，自动选择模式）
	executor.MustRegister(NewReActAgentExecutor())     // ai_react（纯 ReAct 模式）
	executor.MustRegister(NewPlanAgentExecutor())      // ai_plan（纯 Plan 模式）
	executor.MustRegister(NewDirectAgentExecutor())    // ai_direct（纯 Direct 模式）
}
