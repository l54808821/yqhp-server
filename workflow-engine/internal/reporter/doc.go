// Package reporter 提供工作流执行引擎的报告框架。
//
// reporter 包实现了一个可插拔的报告系统，支持多个并发报告器，
// 用于将执行指标输出到各种目标，如控制台、文件、Prometheus、InfluxDB 和 Webhook。
//
// # 架构
//
// 该包由三个主要组件组成：
//
//   - Reporter: 所有报告器必须实现的接口
//   - Registry: 管理报告器类型注册和工厂函数
//   - Manager: 协调执行的多个报告器
//
// # 使用方法
//
// 使用报告系统：
//
//  1. 创建 Registry 并注册报告器工厂
//  2. 使用 registry 创建 Manager
//  3. 通过配置或直接添加报告器
//  4. 调用 Report() 将指标发送到所有报告器
//  5. 完成后调用 Close() 释放资源
//
// 示例：
//
//	registry := reporter.NewRegistry()
//	registry.Register(reporter.ReporterTypeConsole, console.NewFactory())
//
//	manager := reporter.NewManager(registry)
//	manager.AddReporterFromConfig(ctx, &reporter.ReporterConfig{
//	    Type:    reporter.ReporterTypeConsole,
//	    Enabled: true,
//	})
//
//	manager.Start(ctx)
//	manager.Report(ctx, metrics)
//	manager.Close(ctx)
//
// Requirements: 9.1.1
package reporter
