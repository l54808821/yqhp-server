package jsscript

import (
	"yqhp/workflow-engine/internal/keyword"
)

// RegisterJsScript registers the js_script keyword.
func RegisterJsScript(registry *keyword.Registry) {
	registry.MustRegister(JsScript())
}
