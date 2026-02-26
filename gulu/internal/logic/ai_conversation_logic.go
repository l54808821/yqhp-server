package logic

import (
	"context"
	"encoding/json"
	"errors"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/svc"
)

type AIConversationLogic struct {
	ctx context.Context
}

func NewAIConversationLogic(ctx context.Context) *AIConversationLogic {
	return &AIConversationLogic{ctx: ctx}
}

type CreateConversationReq struct {
	Title     string                 `json:"title"`
	Variables map[string]interface{} `json:"variables"`
}

type UpdateTitleReq struct {
	Title string `json:"title" validate:"required"`
}

type SaveMessageReq struct {
	Role     string                 `json:"role" validate:"required"`
	Content  string                 `json:"content" validate:"required"`
	Metadata map[string]interface{} `json:"metadata"`
}

type ConversationDetail struct {
	model.TAiConversation
	Messages []model.TAiConversationMessage `json:"messages"`
}

func (l *AIConversationLogic) Create(workflowID int64, req *CreateConversationReq, userID int64) (*model.TAiConversation, error) {
	title := req.Title
	if title == "" {
		title = "新的对话"
	}

	var varsJSON *string
	if req.Variables != nil {
		import_json, _ := json.Marshal(req.Variables)
		s := string(import_json)
		varsJSON = &s
	}

	conv := &model.TAiConversation{
		WorkflowID: workflowID,
		Title:      title,
		Variables:  varsJSON,
		CreatedBy:  &userID,
	}

	if err := svc.Ctx.DB.Create(conv).Error; err != nil {
		return nil, err
	}
	return conv, nil
}

func (l *AIConversationLogic) List(workflowID int64) ([]model.TAiConversation, error) {
	var list []model.TAiConversation
	err := svc.Ctx.DB.Where("workflow_id = ?", workflowID).
		Order("updated_at DESC").
		Find(&list).Error
	return list, err
}

func (l *AIConversationLogic) GetDetail(conversationID int64) (*ConversationDetail, error) {
	var conv model.TAiConversation
	if err := svc.Ctx.DB.First(&conv, conversationID).Error; err != nil {
		return nil, err
	}

	var messages []model.TAiConversationMessage
	svc.Ctx.DB.Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&messages)

	return &ConversationDetail{
		TAiConversation: conv,
		Messages:        messages,
	}, nil
}

func (l *AIConversationLogic) Delete(conversationID int64) error {
	tx := svc.Ctx.DB.Begin()
	if err := tx.Where("conversation_id = ?", conversationID).
		Delete(&model.TAiConversationMessage{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(&model.TAiConversation{}, conversationID).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (l *AIConversationLogic) UpdateTitle(conversationID int64, title string) error {
	if title == "" {
		return errors.New("标题不能为空")
	}
	return svc.Ctx.DB.Model(&model.TAiConversation{}).
		Where("id = ?", conversationID).
		Update("title", title).Error
}

func (l *AIConversationLogic) SaveMessage(conversationID int64, req *SaveMessageReq) (*model.TAiConversationMessage, error) {
	if req.Role == "" || req.Content == "" {
		return nil, errors.New("role 和 content 不能为空")
	}

	var metaJSON *string
	if req.Metadata != nil {
		b, _ := json.Marshal(req.Metadata)
		s := string(b)
		metaJSON = &s
	}

	msg := &model.TAiConversationMessage{
		ConversationID: conversationID,
		Role:           req.Role,
		Content:        req.Content,
		Metadata:       metaJSON,
	}

	if err := svc.Ctx.DB.Create(msg).Error; err != nil {
		return nil, err
	}

	// 更新会话的 updated_at
	svc.Ctx.DB.Model(&model.TAiConversation{}).
		Where("id = ?", conversationID).
		Update("updated_at", msg.CreatedAt)

	return msg, nil
}
