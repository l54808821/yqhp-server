package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"yqhp/workflow-engine/internal/config"
	"yqhp/workflow-engine/internal/executor"
	_ "yqhp/workflow-engine/internal/executor/ai" // 注册 AI 执行器
	"yqhp/workflow-engine/internal/slave"
	"yqhp/workflow-engine/pkg/types"
)

var (
	// slave start 命令的 flags
	slaveID           string
	slaveType         string
	slaveAddress      string
	slaveMasterAddr   string
	slaveMaxVUs       int
	slaveCapabilities string
	slaveLabels       string
)

// slaveCmd 是 slave 子命令
var slaveCmd = &cobra.Command{
	Use:   "slave",
	Short: "管理 Slave 节点",
	Long:  `Slave 节点负责实际执行工作流任务。`,
}

// slaveStartCmd 是 slave start 子命令
var slaveStartCmd = &cobra.Command{
	Use:   "start",
	Short: "启动 Slave 节点",
	Long: `启动 Slave 节点，连接到 Master 并等待任务分配。

Slave 节点类型：
  - worker: 执行工作流任务的工作节点
  - gateway: 网关节点，用于流量分发
  - aggregator: 聚合节点，用于指标聚合`,
	Example: `  # 使用默认配置启动
  workflow-engine slave start

  # 指定 Master 地址
  workflow-engine slave start --master localhost:9090

  # 指定 Slave ID 和类型
  workflow-engine slave start --id slave-1 --type worker

  # 指定最大 VU 数和能力
  workflow-engine slave start --max-vus 200 --capabilities http_executor,script_executor

  # 添加标签
  workflow-engine slave start --labels region=cn-east,env=prod`,
	RunE: runSlaveStart,
}

// slaveStatusCmd 是 slave status 子命令
var slaveStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "查看 Slave 节点状态",
	Long:    `查看 Slave 节点的运行状态、当前负载等信息。`,
	Example: `  workflow-engine slave status --address http://localhost:9091`,
	RunE:    runSlaveStatus,
}

func init() {
	rootCmd.AddCommand(slaveCmd)
	slaveCmd.AddCommand(slaveStartCmd)
	slaveCmd.AddCommand(slaveStatusCmd)

	// slave start flags
	slaveStartCmd.Flags().StringVar(&slaveID, "id", "", "Slave ID（不指定则自动生成）")
	slaveStartCmd.Flags().StringVar(&slaveType, "type", "worker", "Slave 类型 (worker, gateway, aggregator)")
	slaveStartCmd.Flags().StringVar(&slaveAddress, "address", ":9091", "Slave 监听地址")
	slaveStartCmd.Flags().StringVar(&slaveMasterAddr, "master", "localhost:9090", "Master 节点地址")
	slaveStartCmd.Flags().IntVar(&slaveMaxVUs, "max-vus", 100, "最大虚拟用户数")
	slaveStartCmd.Flags().StringVar(&slaveCapabilities, "capabilities", "http_executor,script_executor", "能力列表，逗号分隔")
	slaveStartCmd.Flags().StringVar(&slaveLabels, "labels", "", "标签，key=value 格式，逗号分隔")

	// slave status flags
	slaveStatusCmd.Flags().StringVar(&slaveAddress, "address", "http://localhost:9091", "Slave 节点地址")
}

func runSlaveStart(cmd *cobra.Command, args []string) error {
	// 加载配置
	loader := config.NewLoader()
	if cfgFile != "" {
		loader = loader.WithConfigPath(cfgFile)
	}

	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 生成 Slave ID
	id := slaveID
	if id == "" {
		id = fmt.Sprintf("slave-%s", uuid.New().String()[:8])
	}

	// 解析 Slave 类型
	var sType types.SlaveType
	switch slaveType {
	case "worker":
		sType = types.SlaveTypeWorker
	case "gateway":
		sType = types.SlaveTypeGateway
	case "aggregator":
		sType = types.SlaveTypeAggregator
	default:
		return fmt.Errorf("无效的 Slave 类型: %s", slaveType)
	}

	// 解析能力列表
	caps := parseCapabilities(slaveCapabilities)

	// 解析标签
	lbls := parseSlaveLabels(slaveLabels)

	// 应用命令行参数覆盖
	if cmd.Flags().Changed("master") {
		cfg.Slave.MasterAddr = slaveMasterAddr
	}
	if cmd.Flags().Changed("max-vus") {
		cfg.Slave.MaxVUs = slaveMaxVUs
	}

	// 创建 Slave 配置
	slaveCfg := &slave.Config{
		ID:                id,
		Type:              sType,
		Address:           slaveAddress,
		MasterAddress:     cfg.Slave.MasterAddr,
		Capabilities:      caps,
		Labels:            lbls,
		HeartbeatInterval: 5 * time.Second,
		HeartbeatTimeout:  10 * time.Second,
		MaxVUs:            cfg.Slave.MaxVUs,
		CPUCores:          4,
		MemoryMB:          4096,
	}

	// 创建执行器注册中心
	registry := executor.NewRegistry()

	// 创建 Slave
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

	// 打印启动信息
	if !quiet {
		fmt.Printf(Banner, Version)
		fmt.Println()
		fmt.Printf("  正在启动 Slave 节点...\n")
		fmt.Printf("  ID: %s\n", id)
		fmt.Printf("  类型: %s\n", sType)
		fmt.Printf("  地址: %s\n", slaveAddress)
		fmt.Printf("  Master: %s\n", cfg.Slave.MasterAddr)
		fmt.Printf("  最大 VU 数: %d\n", cfg.Slave.MaxVUs)
		fmt.Printf("  能力: %v\n", caps)
		if len(lbls) > 0 {
			fmt.Printf("  标签: %v\n", lbls)
		}
		fmt.Println()
	}

	if err := s.Start(ctx); err != nil {
		return fmt.Errorf("启动 Slave 失败: %w", err)
	}

	// 连接到 Master
	if !quiet {
		fmt.Printf("正在连接 Master: %s...\n", cfg.Slave.MasterAddr)
	}
	if err := s.Connect(ctx, cfg.Slave.MasterAddr); err != nil {
		fmt.Printf("警告: 连接 Master 失败: %v\n", err)
		fmt.Println("Slave 将继续运行并重试连接...")
	} else if !quiet {
		fmt.Println("已成功连接到 Master。")
	}

	if !quiet {
		fmt.Println("Slave 节点已启动。按 Ctrl+C 停止。")
	}

	// 等待上下文取消
	<-ctx.Done()

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := s.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("停止 Slave 失败: %w", err)
	}

	if !quiet {
		fmt.Println("Slave 节点已停止。")
	}
	return nil
}

func runSlaveStatus(cmd *cobra.Command, args []string) error {
	fmt.Printf("正在检查 Slave 状态: %s...\n", slaveAddress)
	fmt.Println()
	fmt.Println("Slave 状态:")
	fmt.Println("  状态: 未知 (未连接)")
	fmt.Println()
	fmt.Println("提示: 请确保 Slave 正在运行且可访问。")
	fmt.Printf("      尝试: curl %s/status\n", slaveAddress)

	return nil
}

func parseCapabilities(s string) []string {
	if s == "" {
		return []string{}
	}
	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func parseSlaveLabels(s string) map[string]string {
	if s == "" {
		return map[string]string{}
	}
	result := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}
