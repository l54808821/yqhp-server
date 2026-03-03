package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"yqhp/gulu/internal/logic"

	"github.com/mark3labs/mcp-go/mcp"
)

func handleListWorkflows(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := int64(req.GetInt("project_id", 0))
	if projectID <= 0 {
		return mcp.NewToolResultError("project_id 是必填参数"), nil
	}

	name := req.GetString("name", "")
	page := req.GetInt("page", 1)
	pageSize := req.GetInt("page_size", 10)

	wfLogic := logic.NewWorkflowLogic(ctx)
	list, total, err := wfLogic.List(&logic.WorkflowListReq{
		Page:      page,
		PageSize:  pageSize,
		ProjectID: projectID,
		Name:      name,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("查询失败: %v", err)), nil
	}

	type workflowItem struct {
		ID           int64  `json:"id"`
		Name         string `json:"name"`
		Description  string `json:"description"`
		Status       int32  `json:"status"`
		Version      int32  `json:"version"`
		WorkflowType string `json:"workflow_type"`
	}

	items := make([]workflowItem, 0, len(list))
	for _, wf := range list {
		item := workflowItem{
			ID:   wf.ID,
			Name: wf.Name,
		}
		if wf.Description != nil {
			item.Description = *wf.Description
		}
		if wf.Status != nil {
			item.Status = *wf.Status
		}
		if wf.Version != nil {
			item.Version = *wf.Version
		}
		if wf.WorkflowType != nil {
			item.WorkflowType = *wf.WorkflowType
		}
		items = append(items, item)
	}

	result := map[string]interface{}{
		"total": total,
		"page":  page,
		"list":  items,
	}
	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

func handleGetWorkflow(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID := int64(req.GetInt("workflow_id", 0))
	if workflowID <= 0 {
		return mcp.NewToolResultError("workflow_id 是必填参数"), nil
	}

	wfLogic := logic.NewWorkflowLogic(ctx)
	wf, err := wfLogic.GetByID(workflowID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("工作流不存在: %v", err)), nil
	}

	type workflowDetail struct {
		ID           int64  `json:"id"`
		Name         string `json:"name"`
		Description  string `json:"description"`
		Status       int32  `json:"status"`
		Version      int32  `json:"version"`
		WorkflowType string `json:"workflow_type"`
		ProjectID    int64  `json:"project_id"`
		Definition   string `json:"definition"`
	}

	detail := workflowDetail{
		ID:        wf.ID,
		Name:      wf.Name,
		ProjectID: wf.ProjectID,
	}
	if wf.Description != nil {
		detail.Description = *wf.Description
	}
	if wf.Status != nil {
		detail.Status = *wf.Status
	}
	if wf.Version != nil {
		detail.Version = *wf.Version
	}
	if wf.WorkflowType != nil {
		detail.WorkflowType = *wf.WorkflowType
	}
	// 截断 definition 避免内容过大
	if len(wf.Definition) > 5000 {
		detail.Definition = wf.Definition[:5000] + "...(truncated)"
	} else {
		detail.Definition = wf.Definition
	}

	data, _ := json.Marshal(detail)
	return mcp.NewToolResultText(string(data)), nil
}

func handleListExecutionRecords(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID := int64(req.GetInt("project_id", 0))
	sourceID := int64(req.GetInt("source_id", 0))
	status := req.GetString("status", "")
	page := req.GetInt("page", 1)
	pageSize := req.GetInt("page_size", 10)

	execLogic := logic.NewExecutionLogic(ctx)
	list, total, err := execLogic.List(&logic.ExecutionListReq{
		Page:      page,
		PageSize:  pageSize,
		ProjectID: projectID,
		SourceID:  sourceID,
		Status:    status,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("查询失败: %v", err)), nil
	}

	type execItem struct {
		ID          int64  `json:"id"`
		ExecutionID string `json:"execution_id"`
		Title       string `json:"title"`
		Status      string `json:"status"`
		Mode        string `json:"mode"`
		SourceType  string `json:"source_type"`
		SourceID    int64  `json:"source_id"`
		ProjectID   int64  `json:"project_id"`
		Duration    *int64 `json:"duration"`
	}

	items := make([]execItem, 0, len(list))
	for _, e := range list {
		items = append(items, execItem{
			ID:          e.ID,
			ExecutionID: e.ExecutionID,
			Title:       e.Title,
			Status:      e.Status,
			Mode:        e.Mode,
			SourceType:  e.SourceType,
			SourceID:    e.SourceID,
			ProjectID:   e.ProjectID,
			Duration:    e.Duration,
		})
	}

	result := map[string]interface{}{
		"total": total,
		"page":  page,
		"list":  items,
	}
	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

func handleListProjects(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	page := req.GetInt("page", 1)
	pageSize := req.GetInt("page_size", 10)

	projectLogic := logic.NewProjectLogic(ctx)
	list, total, err := projectLogic.List(&logic.ProjectListReq{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("查询失败: %v", err)), nil
	}

	type projectItem struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      int32  `json:"status"`
	}

	items := make([]projectItem, 0, len(list))
	for _, p := range list {
		item := projectItem{
			ID:   p.ID,
			Name: p.Name,
		}
		if p.Description != nil {
			item.Description = *p.Description
		}
		if p.Status != nil {
			item.Status = *p.Status
		}
		items = append(items, item)
	}

	result := map[string]interface{}{
		"total": total,
		"page":  page,
		"list":  items,
	}
	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}
