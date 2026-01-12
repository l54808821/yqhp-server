// Package cmd 提供 workflow-engine CLI 的命令实现
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	// 导入所有输出插件
	_ "yqhp/workflow-engine/pkg/output/all"
)

const (
	// Version 是当前版本号
	Version = "0.1.0"
	// Banner 是启动时显示的 ASCII 艺术
	Banner = `
          /\      |‾‾| Workflow Engine %s
     /\  /  \     |  |
    /  \/    \    |  |
   /          \   |  |
  / __________ \  |__|
`
)

var (
	// 全局配置
	cfgFile string
	debug   bool
	quiet   bool
)

// rootCmd 是根命令
var rootCmd = &cobra.Command{
	Use:   "workflow-engine",
	Short: "分布式工作流执行引擎",
	Long: `workflow-engine 是一个高性能的分布式工作流执行引擎，
支持压力测试、性能测试和自动化测试场景。专注于工作流编排和分布式执行。`,
	Version: Version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 全局初始化逻辑
	},
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// 全局 flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "启用调试日志")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "静默模式")

	// 禁用默认的 completion 命令
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// 自定义版本模板
	rootCmd.SetVersionTemplate(fmt.Sprintf(Banner, Version) + "\n")
}

// GetRootCmd 返回根命令（用于测试）
func GetRootCmd() *cobra.Command {
	return rootCmd
}
