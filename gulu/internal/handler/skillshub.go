package handler

import (
	"yqhp/gulu/internal/logic"
	"yqhp/common/response"

	"github.com/gofiber/fiber/v2"
)

// SkillshubSearch 搜索 skills.sh 市场
// GET /api/skillshub/search?q=xxx
func SkillshubSearch(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return response.Error(c, "搜索关键词不能为空")
	}

	hub := logic.NewSkillshubLogic(c.UserContext())
	results, err := hub.Search(query)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, results)
}

// SkillshubDetail 获取 skill 详情
// GET /api/skillshub/detail?path=owner/repo/skill-name
func SkillshubDetail(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return response.Error(c, "Skill 路径不能为空")
	}

	hub := logic.NewSkillshubLogic(c.UserContext())
	detail, err := hub.GetDetail(path)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, detail)
}

// SkillshubInstall 从 skills.sh / GitHub 安装 skill
// POST /api/skillshub/install
func SkillshubInstall(c *fiber.Ctx) error {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.Path == "" {
		return response.Error(c, "Skill 路径不能为空")
	}

	hub := logic.NewSkillshubLogic(c.UserContext())
	result, err := hub.Install(req.Path)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}

// SkillshubInstallFromURL 从 URL 导入 SKILL.md
// POST /api/skillshub/install-url
func SkillshubInstallFromURL(c *fiber.Ctx) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, "参数解析失败")
	}
	if req.URL == "" {
		return response.Error(c, "URL 不能为空")
	}

	hub := logic.NewSkillshubLogic(c.UserContext())
	result, err := hub.InstallFromURL(req.URL)
	if err != nil {
		return response.Error(c, err.Error())
	}

	return response.Success(c, result)
}
