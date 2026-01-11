// Package all 导入所有输出插件
// 在 main 包中导入此包以注册所有输出类型
package all

import (
	_ "yqhp/workflow-engine/pkg/output/console"
	_ "yqhp/workflow-engine/pkg/output/influxdb"
	_ "yqhp/workflow-engine/pkg/output/json"
	_ "yqhp/workflow-engine/pkg/output/kafka"
)
