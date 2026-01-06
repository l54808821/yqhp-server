// Package slave 提供管理 Slave 节点的 CLI 命令
package slave

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"yqhp/workflow-engine/internal/config"
	"yqhp/workflow-engine/internal/executor"
	"yqhp/workflow-engine/internal/slave"
	"yqhp/workflow-engine/pkg/types"

	"github.com/google/uuid"
)

// Execute 执行 slave 命令
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
		return fmt.Errorf("未知的 slave 子命令: %s", subcommand)
	}
}

func printUsage() {
	fmt.Println(`workflow-engine slave - 管理 Slave 节点

用法:
  workflow-engine slave <子命令> [选项]

子命令:
  start     启动 Slave 节点
  status    查看 Slave 节点状态

使用 "workflow-engine slave <子命令> --help" 获取更多信息。`)
}

// executeStart 启动 Slave 节点
func executeStart(args []string) error {
	fs := flag.NewFlagSet("slave start", flag.ExitOnError)

	// 配置选项
	configPath := fs.String("config", "", "配置文件路径")
	slaveID := fs.String("id", "", "Slave ID（不指定则自动生成）")
	slaveType := fs.String("type", "worker", "Slave 类型 (worker, gateway, aggregator)")
	address := fs.String("address", ":9091", "Slave 监听地址")
	masterAddr := fs.String("master", "localhost:9090", "Master 节点地址")
	maxVUs := fs.Int("max-vus", 100, "最大虚拟用户数")
	capabilities := fs.String("capabilities", "http_executor,script_executor", "能力列表，逗号分隔")
	labels := fs.String("labels", "", "标签，key=value 格式，逗号分隔 (如 region=cn-east,env=prod)")

	fs.Usage = func() {
		fmt.Println(`workflow-engine slave start - 启动 Slave 节点

用法:
  workflow-engine slave start [选项]

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

	// 如果未指定则生成 Slave ID
	id := *slaveID
	if id == "" {
		id = fmt.Sprintf("slave-%s", uuid.New().String()[:8])
	}

	// 解析 Slave 类型
	var sType types.SlaveType
	switch *slaveType {
	case "worker":
		sType = types.SlaveTypeWorker
	case "gateway":
		sType = types.SlaveTypeGateway
	case "aggregator":
		sType = types.SlaveTypeAggregator
	default:
		return fmt.Errorf("无效的 Slave 类型: %s", *slaveType)
	}

	// 解析能力列表
	caps := parseCommaSeparated(*capabilities)

	// 解析标签
	lbls := parseLabels(*labels)

	// 应用命令行参数覆盖
	if *masterAddr != "localhost:9090" {
		cfg.Slave.MasterAddr = *masterAddr
	}
	if *maxVUs != 100 {
		cfg.Slave.MaxVUs = *maxVUs
	}

	// 创建 Slave 配置
	slaveCfg := &slave.Config{
		ID:                id,
		Type:              sType,
		Address:           *address,
		MasterAddress:     cfg.Slave.MasterAddr,
		Capabilities:      caps,
		Labels:            lbls,
		HeartbeatInterval: 5 * time.Second,
		HeartbeatTimeout:  10 * time.Second,
		MaxVUs:            cfg.Slave.MaxVUs,
		CPUCores:          4,    // 可从运行时检测
		MemoryMB:          4096, // 可从运行时检测
	}

	// 创建执行器注册中心
	registry := executor.NewRegistry()

	// 创建并启动 Slave
	s := slave.NewWorkerSlave(slaveCfg, registry)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理关闭信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\n正在关闭 Slave...")
		cancel()
	}()

	fmt.Printf("正在启动 Slave 节点...\n")
	fmt.Printf("  ID: %s\n", id)
	fmt.Printf("  类型: %s\n", sType)
	fmt.Printf("  地址: %s\n", *address)
	fmt.Printf("  Master: %s\n", cfg.Slave.MasterAddr)
	fmt.Printf("  最大 VU 数: %d\n", cfg.Slave.MaxVUs)
	fmt.Printf("  能力: %v\n", caps)
	if len(lbls) > 0 {
		fmt.Printf("  标签: %v\n", lbls)
	}
	fmt.Println()

	if err := s.Start(ctx); err != nil {
		return fmt.Errorf("启动 Slave 失败: %w", err)
	}

	// 连接到 Master
	fmt.Printf("正在连接 Master: %s...\n", cfg.Slave.MasterAddr)
	if err := s.Connect(ctx, cfg.Slave.MasterAddr); err != nil {
		fmt.Printf("警告: 连接 Master 失败: %v\n", err)
		fmt.Println("Slave 将继续运行并重试连接...")
	} else {
		fmt.Println("已成功连接到 Master。")
	}

	fmt.Println("Slave 节点已启动。按 Ctrl+C 停止。")

	// 等待上下文取消
	<-ctx.Done()

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := s.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("停止 Slave 失败: %w", err)
	}

	fmt.Println("Slave 节点已停止。")
	return nil
}

// executeStatus 查看 Slave 节点状态
func executeStatus(args []string) error {
	fs := flag.NewFlagSet("slave status", flag.ExitOnError)

	address := fs.String("address", "http://localhost:9091", "Slave 节点地址")

	fs.Usage = func() {
		fmt.Println(`workflow-engine slave status - 查看 Slave 节点状态

用法:
  workflow-engine slave status [选项]

选项:`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("正在检查 Slave 状态: %s...\n", *address)

	fmt.Println()
	fmt.Println("Slave 状态:")
	fmt.Println("  状态: 未知 (未连接)")
	fmt.Println()
	fmt.Println("提示: 请确保 Slave 正在运行且可访问。")
	fmt.Printf("      尝试: curl %s/status\n", *address)

	return nil
}

// parseCommaSeparated 解析逗号分隔的字符串为切片
func parseCommaSeparated(s string) []string {
	if s == "" {
		return []string{}
	}
	result := []string{}
	for _, item := range splitAndTrim(s, ",") {
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// parseLabels 解析逗号分隔的 key=value 字符串为 map
func parseLabels(s string) map[string]string {
	if s == "" {
		return map[string]string{}
	}
	result := make(map[string]string)
	for _, pair := range splitAndTrim(s, ",") {
		parts := splitAndTrim(pair, "=")
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// splitAndTrim 分割字符串并去除每部分的空白
func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, part := range split(s, sep) {
		trimmed := trim(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// split 按分隔符分割字符串
func split(s, sep string) []string {
	result := []string{}
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

// trim 去除字符串首尾空白
func trim(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
