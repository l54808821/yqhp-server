package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"
)

type SkillshubLogic struct {
	ctx    context.Context
	client *http.Client
}

func NewSkillshubLogic(ctx context.Context) *SkillshubLogic {
	return &SkillshubLogic{
		ctx:    ctx,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type SkillshubSearchResult struct {
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Description    string `json:"description"`
	Repository     string `json:"repository"`
	Owner          string `json:"owner"`
	Installs       int    `json:"installs"`
	WeeklyInstalls int    `json:"weekly_installs"`
	Stars          int    `json:"stars"`
	FirstSeen      string `json:"first_seen"`
	SkillPath      string `json:"skill_path"`
}

type SkillshubDetail struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Repository     string `json:"repository"`
	Owner          string `json:"owner"`
	Installs       int    `json:"installs"`
	WeeklyInstalls int    `json:"weekly_installs"`
	Stars          int    `json:"stars"`
	FirstSeen      string `json:"first_seen"`
	SkillMDContent string `json:"skill_md_content"`
	SkillMDPreview string `json:"skill_md_preview"`
	SkillPath      string `json:"skill_path"`
	InstallCommand string `json:"install_command"`
}

// Search searches GitHub for repositories matching the query + "skills SKILL.md".
// Uses the public GitHub Search Repositories API (no auth required, 10 req/min).
func (l *SkillshubLogic) Search(q string) ([]SkillshubSearchResult, error) {
	searchQuery := fmt.Sprintf("%s skills SKILL.md", q)
	apiURL := fmt.Sprintf("https://api.github.com/search/repositories?q=%s&sort=stars&per_page=15", url.QueryEscape(searchQuery))
	req, err := http.NewRequestWithContext(l.ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub 搜索失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API 返回 %d: %s", resp.StatusCode, string(body))
	}

	var searchResp struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			Owner       struct {
				Login string `json:"login"`
			} `json:"owner"`
			StargazersCount int    `json:"stargazers_count"`
			HTMLURL         string `json:"html_url"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}

	var results []SkillshubSearchResult
	for _, item := range searchResp.Items {
		results = append(results, SkillshubSearchResult{
			Name:        filepath.Base(item.FullName),
			Slug:        filepath.Base(item.FullName),
			Description: item.Description,
			Repository:  item.FullName,
			Owner:       item.Owner.Login,
			Stars:       item.StargazersCount,
			SkillPath:   item.FullName,
		})
	}
	return results, nil
}

func (l *SkillshubLogic) GetDetail(skillPath string) (*SkillshubDetail, error) {
	owner, repo, skillName, err := parseSkillPath(skillPath)
	if err != nil {
		return nil, err
	}
	skillDir, skillMDContent, err := l.findSkillMD(owner, repo, skillName)
	if err != nil {
		return nil, fmt.Errorf("获取 SKILL.md 失败: %w", err)
	}
	_ = skillDir
	fm, body, _ := parseSkillMD(skillMDContent)
	detail := &SkillshubDetail{
		Repository:     fmt.Sprintf("%s/%s", owner, repo),
		Owner:          owner,
		SkillPath:      skillPath,
		SkillMDContent: skillMDContent,
		InstallCommand: fmt.Sprintf("npx skills add https://github.com/%s/%s --skill %s", owner, repo, skillName),
	}
	if fm != nil {
		detail.Name = fm.Name
		detail.Description = fm.Description
	} else {
		detail.Name = skillName
	}
	if len(body) > 500 {
		detail.SkillMDPreview = body[:500] + "..."
	} else {
		detail.SkillMDPreview = body
	}
	return detail, nil
}

// Install downloads all files from GitHub and stores them as resources.
func (l *SkillshubLogic) Install(skillPath string) (*SkillInfo, error) {
	owner, repo, skillName, err := parseSkillPath(skillPath)
	if err != nil {
		return nil, err
	}

	skillDir, _, findErr := l.findSkillMD(owner, repo, skillName)
	if findErr != nil {
		return nil, fmt.Errorf("获取 SKILL.md 失败: %w", findErr)
	}

	allFiles, err := l.listGitHubDirRecursive(owner, repo, skillDir)
	if err != nil || len(allFiles) == 0 {
		return nil, fmt.Errorf("获取 Skill 文件列表失败: %w", err)
	}

	var skillMDContent string
	fileContents := make(map[string]string)

	for _, f := range allFiles {
		content, fetchErr := l.fetchGitHubFile(owner, repo, f.Path)
		if fetchErr != nil {
			continue
		}
		relPath := strings.TrimPrefix(f.Path, skillDir+"/")
		fileContents[relPath] = content
		if relPath == "SKILL.md" {
			skillMDContent = content
		}
	}

	if skillMDContent == "" {
		return nil, errors.New("SKILL.md 未找到")
	}

	fm, _, _ := parseSkillMD(skillMDContent)
	if fm == nil {
		fm = &SkillMDFrontmatter{Name: skillName}
	}

	displayName := slugToDisplayName(fm.Name)
	if displayName == "" {
		displayName = slugToDisplayName(skillName)
	}
	slug := fm.Name
	if slug == "" {
		slug = skillName
	}

	version := "1.0.0"
	var metadataJSON *string
	if len(fm.Metadata) > 0 {
		if v, ok := fm.Metadata["version"]; ok && v != "" {
			version = v
		}
		b, _ := json.Marshal(fm.Metadata)
		s := string(b)
		metadataJSON = &s
	}

	now := time.Now()
	isDelete := false
	skillType := int32(2)
	status := int32(1)
	sourceURL := fmt.Sprintf("https://github.com/%s/%s/tree/main/%s", owner, repo, skillName)
	authorStr := owner

	skill := &model.TSkill{
		CreatedAt:     &now,
		UpdatedAt:     &now,
		IsDelete:      &isDelete,
		Name:          displayName,
		Slug:          &slug,
		Description:   strPtr(fm.Description),
		License:       strPtr(fm.License),
		Compatibility: strPtr(fm.Compatibility),
		MetadataJSON:  metadataJSON,
		AllowedTools:  strPtr(fm.AllowedTools),
		Author:        &authorStr,
		SourceURL:     &sourceURL,
		Type:          &skillType,
		Status:        &status,
		Version:       &version,
	}

	q := query.Use(svc.Ctx.DB)
	if err := q.TSkill.WithContext(l.ctx).Create(skill); err != nil {
		return nil, fmt.Errorf("创建 Skill 失败: %w", err)
	}

	for relPath, content := range fileContents {
		contentType := guessContentType(filepath.Base(relPath))
		size := int32(len(content))
		res := &model.TSkillResource{
			SkillID:     skill.ID,
			Path:        relPath,
			Content:     &content,
			ContentType: &contentType,
			Size:        &size,
		}
		_ = q.TSkillResource.WithContext(l.ctx).Create(res)
	}

	return NewSkillLogic(l.ctx).GetByID(skill.ID)
}

func (l *SkillshubLogic) InstallFromURL(skillURL string) (*SkillInfo, error) {
	req, err := http.NewRequestWithContext(l.ctx, http.MethodGet, skillURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载返回 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	content := string(data)
	fm, _, _ := parseSkillMD(content)
	if fm == nil {
		fm = &SkillMDFrontmatter{Name: "imported-skill"}
	}

	displayName := slugToDisplayName(fm.Name)
	if displayName == "" {
		displayName = "Imported Skill"
	}
	slug := fm.Name
	if slug == "" {
		slug = "imported-skill"
	}

	now := time.Now()
	isDelete := false
	skillType := int32(2)
	status := int32(1)
	version := "1.0.0"

	var metadataJSON *string
	if len(fm.Metadata) > 0 {
		if v, ok := fm.Metadata["version"]; ok && v != "" {
			version = v
		}
		b, _ := json.Marshal(fm.Metadata)
		s := string(b)
		metadataJSON = &s
	}

	skill := &model.TSkill{
		CreatedAt:     &now,
		UpdatedAt:     &now,
		IsDelete:      &isDelete,
		Name:          displayName,
		Slug:          &slug,
		Description:   strPtr(fm.Description),
		License:       strPtr(fm.License),
		Compatibility: strPtr(fm.Compatibility),
		MetadataJSON:  metadataJSON,
		AllowedTools:  strPtr(fm.AllowedTools),
		SourceURL:     &skillURL,
		Type:          &skillType,
		Status:        &status,
		Version:       &version,
	}

	q := query.Use(svc.Ctx.DB)
	if err := q.TSkill.WithContext(l.ctx).Create(skill); err != nil {
		return nil, fmt.Errorf("创建 Skill 失败: %w", err)
	}

	contentType := "text/markdown"
	size := int32(len(content))
	res := &model.TSkillResource{
		SkillID:     skill.ID,
		Path:        "SKILL.md",
		Content:     &content,
		ContentType: &contentType,
		Size:        &size,
	}
	_ = q.TSkillResource.WithContext(l.ctx).Create(res)

	return NewSkillLogic(l.ctx).GetByID(skill.ID)
}

// --- GitHub helpers ---

type githubFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

// findSkillMD tries multiple directory layouts to locate SKILL.md:
//   - {skillName}/SKILL.md          (skills stored in repo root)
//   - skills/{skillName}/SKILL.md   (skills stored in skills/ subdir, e.g. vercel-labs/skills)
//
// Returns (actualDirPath, skillMDContent, error).
func (l *SkillshubLogic) findSkillMD(owner, repo, skillName string) (string, string, error) {
	if skillName == "" {
		content, err := l.fetchGitHubFile(owner, repo, "SKILL.md")
		if err == nil {
			return ".", content, nil
		}
		return "", "", fmt.Errorf("仓库根目录下未找到 SKILL.md")
	}

	candidates := []string{
		skillName,
		"skills/" + skillName,
	}
	for _, dir := range candidates {
		content, err := l.fetchGitHubFile(owner, repo, dir+"/SKILL.md")
		if err == nil {
			return dir, content, nil
		}
	}
	return "", "", fmt.Errorf("在 %s/%s 中未找到 %s 的 SKILL.md（尝试了 %s/SKILL.md 和 skills/%s/SKILL.md）",
		owner, repo, skillName, skillName, skillName)
}

func (l *SkillshubLogic) fetchGitHubFile(owner, repo, path string) (string, error) {
	branch := l.getDefaultBranch(owner, repo)
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, path)
	req, err := http.NewRequestWithContext(l.ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("文件不存在: %s (分支: %s)", path, branch)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub 返回 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// getDefaultBranch queries the GitHub API for the repo's default branch.
// Falls back to "main" if the API call fails.
func (l *SkillshubLogic) getDefaultBranch(owner, repo string) string {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	req, err := http.NewRequestWithContext(l.ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "main"
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := l.client.Do(req)
	if err != nil {
		return "main"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "main"
	}
	var repoInfo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return "main"
	}
	if repoInfo.DefaultBranch == "" {
		return "main"
	}
	return repoInfo.DefaultBranch
}

func (l *SkillshubLogic) listGitHubDir(owner, repo, path string) ([]githubFile, error) {
	branch := l.getDefaultBranch(owner, repo)
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, branch)
	req, err := http.NewRequestWithContext(l.ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	var files []githubFile
	json.NewDecoder(resp.Body).Decode(&files)
	return files, nil
}

func (l *SkillshubLogic) listGitHubDirRecursive(owner, repo, path string) ([]githubFile, error) {
	entries, err := l.listGitHubDir(owner, repo, path)
	if err != nil {
		return nil, err
	}
	var result []githubFile
	for _, e := range entries {
		if e.Type == "file" {
			result = append(result, e)
		} else if e.Type == "dir" {
			sub, _ := l.listGitHubDirRecursive(owner, repo, e.Path)
			result = append(result, sub...)
		}
	}
	return result, nil
}

// parseSkillPath accepts multiple input formats and extracts owner/repo/skillName:
//   - "vercel-labs/skills/find-skills"
//   - "https://skills.sh/vercel-labs/skills/find-skills"
//   - "https://github.com/vercel-labs/skills"
//   - "npx skills add https://github.com/vercel-labs/skills --skill find-skills"
func parseSkillPath(input string) (owner, repo, skillName string, err error) {
	input = strings.TrimSpace(input)

	if strings.HasPrefix(input, "npx skills add") {
		return parseNpxCommand(input)
	}

	input = strings.TrimPrefix(input, "https://skills.sh/")
	input = strings.TrimPrefix(input, "http://skills.sh/")
	input = strings.TrimPrefix(input, "https://github.com/")
	input = strings.TrimPrefix(input, "http://github.com/")
	input = strings.Trim(input, "/")

	parts := strings.Split(input, "/")
	if len(parts) < 2 {
		return "", "", "", errors.New("无效路径，支持格式：owner/repo/skill-name 或 skills.sh URL 或 npx skills add 命令")
	}

	owner = parts[0]
	repo = parts[1]
	if len(parts) >= 3 {
		skillName = strings.Join(parts[2:], "/")
		skillName = strings.TrimSuffix(skillName, "/")
	}
	return
}

func parseNpxCommand(cmd string) (owner, repo, skillName string, err error) {
	parts := strings.Fields(cmd)

	var githubURL string
	for _, p := range parts {
		if strings.Contains(p, "github.com/") {
			githubURL = p
			break
		}
	}

	for i, p := range parts {
		if p == "--skill" && i+1 < len(parts) {
			skillName = parts[i+1]
			break
		}
	}

	if githubURL == "" {
		return "", "", "", errors.New("npx 命令中未找到 GitHub URL")
	}

	githubURL = strings.TrimPrefix(githubURL, "https://github.com/")
	githubURL = strings.TrimPrefix(githubURL, "http://github.com/")
	githubURL = strings.Trim(githubURL, "/")

	urlParts := strings.Split(githubURL, "/")
	if len(urlParts) < 2 {
		return "", "", "", errors.New("GitHub URL 格式无效")
	}

	owner = urlParts[0]
	repo = urlParts[1]
	return
}
