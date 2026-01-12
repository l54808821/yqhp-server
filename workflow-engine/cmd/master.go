package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"yqhp/workflow-engine/internal/config"
	"yqhp/workflow-engine/internal/master"
)

var (
	// master start 命令的 flags
	masterAddress          string
	masterStandalone       bool
	masterHeartbeatTimeout time.Duration
	masterMaxExecutions    int
)

// masterCmd 是 master 子命令
var masterCmd = &cobra.Command{
	Use:   "master",
	Short: "管理 Master 节点",
	Long:  `Master 节点负责工作流调度、任务分发和指标聚合。`,
}

// masterStartCmd 是 master start 子命令
var masterStartCmd = &cobra.Command{
	Use:   "start",
	Short: "启动 Master 节点",
	Long: `启动 Master 节点，开始接受 Slave 连接和工作流提交。

Master 节点是分布式执行的核心，负责：
  - 管理 Slave 节点注册和心跳
  - 调度工作流到合适的 Slave
  - 聚合执行指标
  - 提供 REST API`,
	Example: `  # 使用默认配置启动
  workflow-engine master start

  # 指定监听地址
  workflow-engine master start --address :9090

  # 独立模式（无需 Slave）
  workflow-engine master start --standalone

  # 使用配置文件
  workflow-engine master start --config config.yaml`,
	RunE: runMasterStart,
}

// masterStatusCmd 是 master status 子命令
var masterStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看 Master 节点状态",
	Long:  `查看 Master 节点的运行状态、连接的 Slave 数量等信息。`,
	Example: `  workflow-engine master status
  workflow-engine master status --address http://localhost:9090`,
	RunE: runMasterStatus,
}

func init() {
	rootCmd.AddCommand(masterCmd)
	masterCmd.AddCommand(masterStartCmd)
	masterCmd.AddCommand(masterStatusCmd)

	// master start flags
	masterStartCmd.Flags().StringVar(&masterAddress, "address", ":8080", "HTTP 服务地址")
	masterStartCmd.Flags().BoolVar(&masterStandalone, "standalone", false, "独立模式运行（无需 Slave）")
	masterStartCmd.Flags().DurationVar(&masterHeartbeatTimeout, "heartbeat-timeout", 30*time.Second, "Slave 心跳超时时间")
	masterStartCmd.Flags().IntVar(&masterMaxExecutions, "max-executions", 100, "最大并发执行数")

	// master status flags
	masterStatusCmd.Flags().StringVar(&masterAddress, "address", "http://localhost:8080", "Master 节点地址")
}

func runMasterStart(cmd *cobra.Command, args []string) error {
	// 加载配置
	loader := config.NewLoader()
	if cfgFile != "" {
		loader = loader.WithConfigPath(cfgFile)
	}

	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 应用命令行参数覆盖
	if cmd.Flags().Changed("address") {
		cfg.Server.Address = masterAddress
	}
	if cmd.Flags().Changed("heartbeat-timeout") {
		cfg.Master.HeartbeatTimeout = masterHeartbeatTimeout
	}

	// 创建 Master 配置
	masterCfg := &master.Config{
		Address:                 cfg.Server.Address,
		HeartbeatTimeout:        cfg.Master.HeartbeatTimeout,
		HealthCheckInterval:     cfg.Master.HeartbeatInterval,
		StandaloneMode:          masterStandalone,
		MaxConcurrentExecutions: masterMaxExecutions,
	}

	// 创建组件
	registry := master.NewInMemorySlaveRegistry()
	scheduler := master.NewWorkflowScheduler(registry)
	aggregator := master.NewDefaultMetricsAggregator()

	// 创建 Master
	m := master.NewWorkflowMaster(masterCfg, registry, scheduler, aggregator)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理关闭信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\n正在关闭 Master...")
		cancel()
	}()

	// 打印启动信息
	if !quiet {
		fmt.Printf(Banner, Version)
		fmt.Println()
		fmt.Printf("  正在启动 Master 节点...\n")
		fmt.Printf("  HTTP 地址: %s\n", cfg.Server.Address)
		fmt.Printf("  独立模式: %v\n", masterStandalone)
		fmt.Printf("  最大并发执行数: %d\n", masterMaxExecutions)
		fmt.Println()
	}

	if err := m.Start(ctx); err != nil {
		return fmt.Errorf("启动 Master 失败: %w", err)
	}

	if !quiet {
		fmt.Println("Master 节点启动成功。按 Ctrl+C 停止。")
	}

	// 等待上下文取消
	<-ctx.Done()

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := m.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("停止 Master 失败: %w", err)
	}

	if !quiet {
		fmt.Println("Master 节点已停止。")
	}
	return nil
}

func runMasterStatus(cmd *cobra.Command, args []string) error {
	fmt.Printf("正在检查 Master 状态: %s...\n", masterAddress)
	fmt.Println()
	fmt.Println("Master 状态:")
	fmt.Println("  状态: 未知 (未连接)")
	fmt.Println()
	fmt.Println("提示: 请确保 Master 正在运行且可访问。")
	fmt.Printf("      尝试: curl %s/api/v1/health\n", masterAddress)

	return nil
}
