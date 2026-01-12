package action

import (
	"yqhp/workflow-engine/internal/keyword"
)

// RegisterAllActions registers all action keywords.
func RegisterAllActions(registry *keyword.Registry) {
	RegisterSetVariable(registry)
	RegisterWait(registry)
	RegisterDBQuery(registry)
}
