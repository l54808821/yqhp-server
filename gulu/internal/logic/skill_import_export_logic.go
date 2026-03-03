package logic

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"yqhp/gulu/internal/model"
	"yqhp/gulu/internal/query"
	"yqhp/gulu/internal/svc"

	"gopkg.in/yaml.v3"
)

type SkillImportExportLogic struct {
	ctx context.Context
}

func NewSkillImportExportLogic(ctx context.Context) *SkillImportExportLogic {
	return &SkillImportExportLogic{ctx: ctx}
}

type SkillMDFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license,omitempty"`
	Compatibility string            `yaml:"compatibility,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
	AllowedTools  string            `yaml:"allowed-tools,omitempty"`
}

// Import imports a skill from a zip file. All files (including SKILL.md) go into t_skill_resource.
func (l *SkillImportExportLogic) Import(zipData []byte) (*SkillInfo, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip: %w", err)
	}

	var skillMDContent string
	var skillMDRelPath string
	var rootPrefix string

	for _, f := range reader.File {
		if strings.EqualFold(filepath.Base(f.Name), "SKILL.md") {
			r, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("cannot open SKILL.md: %w", err)
			}
			data, err := io.ReadAll(r)
			r.Close()
			if err != nil {
				return nil, fmt.Errorf("cannot read SKILL.md: %w", err)
			}
			skillMDContent = string(data)
			skillMDRelPath = filepath.ToSlash(f.Name)
			rootPrefix = strings.TrimSuffix(skillMDRelPath, "SKILL.md")
			break
		}
	}

	if skillMDContent == "" {
		return nil, errors.New("SKILL.md not found in zip")
	}

	frontmatter, body, err := parseSkillMD(skillMDContent)
	if err != nil {
		frontmatter = &SkillMDFrontmatter{}
		body = skillMDContent
	}

	name := slugToDisplayName(frontmatter.Name)
	slug := frontmatter.Name
	if frontmatter.Name == "" {
		name = "Unnamed Skill"
		slug = "unnamed-skill"
	}

	version := "1.0.0"
	if frontmatter.Metadata != nil {
		if v, ok := frontmatter.Metadata["version"]; ok && v != "" {
			version = v
		}
	}

	var metadataJSON *string
	if len(frontmatter.Metadata) > 0 {
		b, _ := json.Marshal(frontmatter.Metadata)
		s := string(b)
		metadataJSON = &s
	}

	now := time.Now()
	isDelete := false
	skillType := int32(2)
	status := int32(1)

	skill := &model.TSkill{
		CreatedAt:     &now,
		UpdatedAt:     &now,
		IsDelete:      &isDelete,
		Name:          name,
		Slug:          &slug,
		Description:   strPtr(frontmatter.Description),
		License:       strPtr(frontmatter.License),
		Compatibility: strPtr(frontmatter.Compatibility),
		MetadataJSON:  metadataJSON,
		AllowedTools:  strPtr(frontmatter.AllowedTools),
		Type:          &skillType,
		Status:        &status,
		Version:       &version,
	}

	q := query.Use(svc.Ctx.DB)
	if err := q.TSkill.WithContext(l.ctx).Create(skill); err != nil {
		return nil, fmt.Errorf("create skill: %w", err)
	}

	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		path := filepath.ToSlash(f.Name)
		if rootPrefix != "" {
			path = strings.TrimPrefix(path, rootPrefix)
		}
		if path == "" {
			continue
		}

		r, err := f.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			continue
		}

		contentStr := string(content)
		_ = body // suppress unused warning for the body var from parseSkillMD
		contentType := guessContentType(filepath.Base(path))
		size := int32(len(contentStr))

		res := &model.TSkillResource{
			SkillID:     skill.ID,
			Path:        path,
			Content:     &contentStr,
			ContentType: &contentType,
			Size:        &size,
		}
		_ = q.TSkillResource.WithContext(l.ctx).Create(res)
	}

	return NewSkillLogic(l.ctx).GetByID(skill.ID)
}

// Export exports a skill to a zip file. Reads all files from t_skill_resource.
func (l *SkillImportExportLogic) Export(skillID int64) ([]byte, string, error) {
	q := query.Use(svc.Ctx.DB)

	skill, err := q.TSkill.WithContext(l.ctx).Where(
		q.TSkill.ID.Eq(skillID),
		q.TSkill.IsDelete.Is(false),
	).First()
	if err != nil {
		return nil, "", errors.New("skill not found")
	}

	resources, err := q.TSkillResource.WithContext(l.ctx).
		Where(q.TSkillResource.SkillID.Eq(skillID)).
		Find()
	if err != nil {
		resources = nil
	}

	dirName := sanitizeSlug(skill.Name)
	if skill.Slug != nil && *skill.Slug != "" {
		dirName = *skill.Slug
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	hasSKILLMD := false
	for _, res := range resources {
		filePath := filepath.Join(dirName, res.Path)
		w, err := zipWriter.Create(filePath)
		if err != nil {
			continue
		}
		content := ""
		if res.Content != nil {
			content = *res.Content
		}
		w.Write([]byte(content))
		if res.Path == "SKILL.md" {
			hasSKILLMD = true
		}
	}

	if !hasSKILLMD {
		skillMD := generateSKILLMD(skill, "")
		filePath := filepath.Join(dirName, "SKILL.md")
		if w, err := zipWriter.Create(filePath); err == nil {
			w.Write([]byte(skillMD))
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, "", err
	}

	filename := dirName + ".zip"
	return buf.Bytes(), filename, nil
}

func parseSkillMD(content string) (*SkillMDFrontmatter, string, error) {
	re := regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n(.*)$`)
	matches := re.FindStringSubmatch(content)
	if len(matches) != 3 {
		return nil, "", errors.New("invalid SKILL.md: missing YAML frontmatter")
	}

	var fm SkillMDFrontmatter
	if err := yaml.Unmarshal([]byte(matches[1]), &fm); err != nil {
		return nil, "", fmt.Errorf("invalid SKILL.md frontmatter: %w", err)
	}

	body := strings.TrimSpace(matches[2])
	return &fm, body, nil
}

func slugToDisplayName(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func sanitizeSlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	re := regexp.MustCompile(`[^a-z0-9-]`)
	name = re.ReplaceAllString(name, "")
	re2 := regexp.MustCompile(`-{2,}`)
	name = re2.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "unnamed-skill"
	}
	return name
}
