package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"yqhp/common/response"
	"yqhp/gulu/internal/logic"

	"github.com/gofiber/fiber/v2"
)

// ExecutionGetRealtimeMetrics returns realtime metrics snapshot from the engine.
// GET /api/execution-records/:id/realtime
func ExecutionGetRealtimeMetrics(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行记录ID")
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())
	metrics, err := executionLogic.GetRealtimeMetrics(id)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, metrics)
}

// ExecutionGetReport returns the final performance test report.
// GET /api/execution-records/:id/report
func ExecutionGetReport(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行记录ID")
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())
	report, err := executionLogic.GetReport(id)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, report)
}

// ExecutionGetTimeSeries returns time-series data for charting.
// GET /api/execution-records/:id/timeseries
func ExecutionGetTimeSeries(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行记录ID")
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())
	data, err := executionLogic.GetTimeSeries(id)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, data)
}

// ScaleVUsRequest represents a VU scaling request.
type ScaleVUsRequest struct {
	VUs int `json:"vus" validate:"required,min=1"`
}

// ExecutionScaleVUs adjusts the VU count for a running execution.
// POST /api/execution-records/:id/scale
func ExecutionScaleVUs(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行记录ID")
	}

	var req ScaleVUsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败: "+err.Error())
	}

	if req.VUs < 1 {
		return response.Error(c, "VU数量必须大于0")
	}

	executionLogic := logic.NewExecutionLogic(c.UserContext())
	err = executionLogic.ScaleVUs(id, req.VUs)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, fiber.Map{"vus": req.VUs})
}

// ExecutionMetricsStream provides a Server-Sent Events (SSE) stream of realtime metrics.
// This replaces WebSocket for simplicity and is compatible with the existing SSE infrastructure.
// GET /api/execution-records/:id/metrics/stream
func ExecutionMetricsStream(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.Error(c, "无效的执行记录ID")
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		executionLogic := logic.NewExecutionLogic(c.UserContext())
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				metrics, err := executionLogic.GetRealtimeMetrics(id)
				if err != nil {
					fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
					w.Flush()
					return
				}

				data, _ := json.Marshal(metrics)
				fmt.Fprintf(w, "event: metrics\ndata: %s\n\n", string(data))
				if err := w.Flush(); err != nil {
					return
				}

				// Check if execution has completed
				execution, execErr := executionLogic.GetByID(id)
				if execErr != nil {
					return
				}
				if execution.Status == logic.ExecutionStatusCompleted ||
					execution.Status == logic.ExecutionStatusFailed ||
					execution.Status == logic.ExecutionStatusStopped {

					// Send final report if available
					report, reportErr := executionLogic.GetReport(id)
					if reportErr == nil && report != nil {
						reportData, _ := json.Marshal(report)
						fmt.Fprintf(w, "event: report\ndata: %s\n\n", string(reportData))
					}

					fmt.Fprintf(w, "event: complete\ndata: {\"status\":\"%s\"}\n\n", execution.Status)
					w.Flush()
					return
				}
			}
		}
	})

	return nil
}
