package ai

import "yqhp/workflow-engine/internal/executor"

func init() {
	executor.MustRegister(NewAgentExecutor())
}
