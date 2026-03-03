package ai

import "yqhp/workflow-engine/internal/executor"

func init() {
	unified := NewUnifiedAgentExecutor()
	executor.MustRegister(unified)
}
