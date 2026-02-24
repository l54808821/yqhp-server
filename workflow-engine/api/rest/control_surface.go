package rest

import (
	"yqhp/workflow-engine/pkg/controlsurface"
)

// Re-export types from controlsurface package for convenience in handlers.
type ControlSurface = controlsurface.ControlSurface
type ExecutionStatus = controlsurface.ExecutionStatus
type RealtimeMetrics = controlsurface.RealtimeMetrics

// RegisterControlSurface delegates to the shared registry.
func RegisterControlSurface(executionID string, cs *ControlSurface) {
	controlsurface.Register(executionID, cs)
}

// UnregisterControlSurface delegates to the shared registry.
func UnregisterControlSurface(executionID string) {
	controlsurface.Unregister(executionID)
}

// GetControlSurface delegates to the shared registry.
func GetControlSurface(executionID string) *ControlSurface {
	return controlsurface.Get(executionID)
}
