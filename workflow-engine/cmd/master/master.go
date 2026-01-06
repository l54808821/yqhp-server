// Package master 提供管理 Master 节点的 CLI 命令
package master

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"yqhp/workflow-engine/internal/config"
	"yqhp/workflow-engine/internal/master"
)

// Execute 执行 master 命令
func Execute(args []string) error {
	if len(args) < 1 {
		printUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "start":
		return executeStart(subArgs)
	case "status":
		return executeStatus(subArgs)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("未知的 master 子命令: %s", subcommand)
	}
}

func printUsage() {
	fmt.Println(`workflow-engine master - 管理 Master 节点

用法:
  workflow-engine master <子命令> [选项]

子命令:
  start     启动 Master 节点
  status    查看 Master 节点状态

使用 "workflow-engine master <子命令> --help" 获取更多信息。`)
}

// executeStart 启动 Master 节点
func executeStart(args []string) error {
	fs := flag.NewFlagSet("master start", flag.ExitOnError)

	// 配置选项
	configPath := fs.String("config", "", "配置文件路径")
	address := fs.String("address", ":8080", "HTTP 服务地址")
	grpcAddress := fs.String("grpc-address", ":9090", "gRPC 服务地址")
	standalone := fs.Bool("standalone", false, "独立模式运行（无需 Slave）")
	heartbeatTimeout := fs.Duration("heartbeat-timeout", 30*time.Second, "Slave 心跳超时时间")
	maxExecutions := fs.Int("max-executions", 100, "最大并发执行数")

	fs.Usage = func() {
		fmt.Println(`workflow-engine master start - 启动 Master 节点

用法:
  workflow-engine master start [选项]

选项:`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// 加载配置
	loader := config.NewLoader()
	if *configPath != "" {
		loader = loader.WithConfigPath(*configPath)
	}

	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 应用命令行参数覆盖
	if *address != ":8080" {
		cfg.Server.Address = *address
	}
	if *grpcAddress != ":9090" {
		cfg.GRPC.Address = *grpcAddress
	}
	if *heartbeatTimeout != 30*time.Second {
		cfg.Master.HeartbeatTimeout = *heartbeatTimeout
	}

	// 创建 Master 配置
	masterCfg := &master.Config{
		Address:                 cfg.Server.Address,
		HeartbeatTimeout:        cfg.Master.HeartbeatTimeout,
		HealthCheckInterval:     cfg.Master.HeartbeatInterval,
		StandaloneMode:          *standalone,
		MaxConcurrentExecutions: *maxExecutions,
	}

	// 创建注册中心、调度器和聚合器
	registry := master.NewInMemorySlaveRegistry()
	scheduler := master.NewWorkflowScheduler(registry)
	aggregator := master.NewDefaultMetricsAggregator()

	// 创建并启动 Master
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

	fmt.Printf("正在启动 Master 节点...\n")
	fmt.Printf("  HTTP 地址: %s\n", cfg.Server.Address)
	fmt.Printf("  gRPC 地址: %s\n", cfg.GRPC.Address)
	fmt.Printf("  独立模式: %v\n", *standalone)
	fmt.Printf("  最大并发执行数: %d\n", *maxExecutions)
	fmt.Println()

	if err := m.Start(ctx); err != nil {
		return fmt.Errorf("启动 Master 失败: %w", err)
	}

	fmt.Println("Master 节点启动成功。按 Ctrl+C 停止。")

	// 等待上下文取消
	<-ctx.Done()

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := m.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("停止 Master 失败: %w", err)
	}

	fmt.Println("Master 节点已停止。")
	return nil
}

// executeStatus 查看 Master 节点状态
func executeStatus(args []string) error {
	fs := flag.NewFlagSet("master status", flag.ExitOnError)

	address := fs.String("address", "http://localhost:8080", "Master 节点地址")

	fs.Usage = func() {
		fmt.Println(`workflow-engine master status - 查看 Master 节点状态

用法:
  workflow-engine master status [选项]

选项:`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("正在检查 Master 状态: %s...\n", *address)

	fmt.Println()
	fmt.Println("Master 状态:")
	fmt.Println("  状态: 未知 (未连接)")
	fmt.Println()
	fmt.Println("提示: 请确保 Master 正在运行且可访问。")
	fmt.Printf("      尝试: curl %s/api/v1/health\n", *address)

	return nil
}
