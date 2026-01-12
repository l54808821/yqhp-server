// Package init provides unified keyword registration for workflow engine v2.
package init

import (
	"yqhp/workflow-engine/internal/keyword"
	"yqhp/workflow-engine/internal/keyword/action"
	"yqhp/workflow-engine/internal/keyword/assertion"
	"yqhp/workflow-engine/internal/keyword/extractor"
	"yqhp/workflow-engine/internal/keyword/jsscript"
)

// RegisterAllKeywords registers all keywords to the given registry.
func RegisterAllKeywords(registry *keyword.Registry) {
	// Register assertion keywords
	assertion.RegisterCompareAssertions(registry)
	assertion.RegisterStringAssertions(registry)
	assertion.RegisterCollectionAssertions(registry)
	assertion.RegisterTypeAssertions(registry)

	// Register extractor keywords
	extractor.RegisterAllExtractors(registry)

	// Register action keywords
	action.RegisterAllActions(registry)

	// Register JS script keyword
	jsscript.RegisterJsScript(registry)
}

// InitDefaultRegistry initializes the default registry with all keywords.
func InitDefaultRegistry() {
	RegisterAllKeywords(keyword.DefaultRegistry)
}
