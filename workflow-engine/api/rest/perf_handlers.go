package rest

import (
	"github.com/gofiber/fiber/v2"
)

// setupPerfRoutes registers performance testing API routes.
// These routes follow k6's REST API design pattern.
func (s *Server) setupPerfRoutes() {
	perf := s.app.Group("/api/v1/perf")

	// GET /api/v1/perf/executions/:id/status - Get execution status (VUs, iterations, etc.)
	perf.Get("/executions/:id/status", s.handleGetPerfStatus)

	// PATCH /api/v1/perf/executions/:id/status - Modify execution (scale VUs, pause, stop)
	perf.Patch("/executions/:id/status", s.handlePatchPerfStatus)

	// GET /api/v1/perf/executions/:id/metrics - Get all aggregated metrics
	perf.Get("/executions/:id/metrics", s.handleGetPerfMetrics)

	// GET /api/v1/perf/executions/:id/metrics/realtime - Get realtime metrics snapshot
	perf.Get("/executions/:id/metrics/realtime", s.handleGetRealtimeMetrics)

	// GET /api/v1/perf/executions/:id/report - Get final performance report
	perf.Get("/executions/:id/report", s.handleGetPerfReport)

	// GET /api/v1/perf/executions/:id/timeseries - Get time-series data
	perf.Get("/executions/:id/timeseries", s.handleGetTimeSeries)
}

// handleGetPerfStatus returns the current execution status.
func (s *Server) handleGetPerfStatus(c *fiber.Ctx) error {
	execID := c.Params("id")
	cs := GetControlSurface(execID)
	if cs == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "execution not found or not a performance test",
		})
	}

	if cs.GetStatus != nil {
		return c.JSON(cs.GetStatus())
	}

	return c.JSON(&ExecutionStatus{Status: "unknown"})
}

// PatchStatusRequest represents a status modification request.
type PatchStatusRequest struct {
	VUs     *int  `json:"vus,omitempty"`
	Paused  *bool `json:"paused,omitempty"`
	Stopped *bool `json:"stopped,omitempty"`
}

// handlePatchPerfStatus modifies the execution (scale VUs, pause/resume, stop).
// This mirrors k6's PATCH /v1/status endpoint.
func (s *Server) handlePatchPerfStatus(c *fiber.Ctx) error {
	execID := c.Params("id")
	cs := GetControlSurface(execID)
	if cs == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "execution not found or not a performance test",
		})
	}

	var req PatchStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if req.Stopped != nil && *req.Stopped {
		if cs.StopExecution != nil {
			if err := cs.StopExecution(); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
			}
		}
	}

	if req.Paused != nil {
		if *req.Paused {
			if cs.PauseExecution != nil {
				if err := cs.PauseExecution(); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
		} else {
			if cs.ResumeExecution != nil {
				if err := cs.ResumeExecution(); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
				}
			}
		}
	}

	if req.VUs != nil {
		if cs.ScaleVUs != nil {
			if err := cs.ScaleVUs(*req.VUs); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
			}
		}
		if cs.SummaryOutput != nil {
			cs.SummaryOutput.RecordVUChange(*req.VUs, "manual", "API request")
		}
	}

	if cs.GetStatus != nil {
		return c.JSON(cs.GetStatus())
	}
	return c.JSON(fiber.Map{"success": true})
}

// handleGetPerfMetrics returns all aggregated metrics.
func (s *Server) handleGetPerfMetrics(c *fiber.Ctx) error {
	execID := c.Params("id")
	cs := GetControlSurface(execID)
	if cs == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "execution not found",
		})
	}

	if cs.MetricsEngine == nil {
		return c.JSON(fiber.Map{})
	}

	status := "running"
	if cs.GetStatus != nil {
		status = cs.GetStatus().Status
	}
	return c.JSON(cs.MetricsEngine.BuildRealtimeMetrics(status, cs.GetVUs, cs.GetIterations, cs.GetErrors))
}

// handleGetRealtimeMetrics returns a realtime metrics snapshot for live dashboards.
func (s *Server) handleGetRealtimeMetrics(c *fiber.Ctx) error {
	execID := c.Params("id")
	cs := GetControlSurface(execID)
	if cs == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "execution not found",
		})
	}

	if cs.MetricsEngine == nil {
		return c.JSON(fiber.Map{})
	}

	status := "running"
	if cs.GetStatus != nil {
		s := cs.GetStatus()
		status = s.Status
	}

	rm := cs.MetricsEngine.BuildRealtimeMetrics(
		status,
		cs.GetVUs,
		cs.GetIterations,
		cs.GetErrors,
	)

	return c.JSON(rm)
}

// handleGetPerfReport returns the final performance test report.
func (s *Server) handleGetPerfReport(c *fiber.Ctx) error {
	execID := c.Params("id")
	cs := GetControlSurface(execID)
	if cs == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "execution not found",
		})
	}

	if cs.FinalReport != nil {
		return c.JSON(cs.FinalReport)
	}

	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": "report not yet available (test still running)",
	})
}

// handleGetTimeSeries returns the time-series data for charting.
func (s *Server) handleGetTimeSeries(c *fiber.Ctx) error {
	execID := c.Params("id")
	cs := GetControlSurface(execID)
	if cs == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "execution not found",
		})
	}

	if cs.MetricsEngine == nil {
		return c.JSON([]interface{}{})
	}

	return c.JSON(cs.MetricsEngine.GetTimeSeriesData())
}
