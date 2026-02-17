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

// SkillImportExportLogic handles import/export of Agent Skills standard format
type SkillImportExportLogic struct {
	ctx context.Context
}

// NewSkillImportExportLogic creates a new SkillImportExportLogic
func NewSkillImportExportLogic(ctx context.Context) *SkillImportExportLogic {
	return &SkillImportExportLogic{ctx: ctx}
}

// SkillMDFrontmatter represents the YAML frontmatter in SKILL.md
type SkillMDFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license,omitempty"`
	Compatibility string            `yaml:"compatibility,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
	AllowedTools  string            `yaml:"allowed-tools,omitempty"`
}

// Import imports a skill from a zip file containing SKILL.md and optional resource files
func (l *SkillImportExportLogic) Import(zipData []byte) (*SkillInfo, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip: %w", err)
	}

	// Find SKILL.md (could be at root or in a subdirectory)
	var skillMDContent string
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
			break
		}
	}

	if skillMDContent == "" {
		return nil, errors.New("SKILL.md not found in zip")
	}

	frontmatter, body, err := parseSkillMD(skillMDContent)
	if err != nil {
		return nil, err
	}

	// Determine name and slug
	name := slugToDisplayName(frontmatter.Name)
	slug := frontmatter.Name
	if frontmatter.Name == "" {
		name = "Unnamed Skill"
		slug = "unnamed-skill"
	}

	// Version from metadata or default
	version := "1.0.0"
	if frontmatter.Metadata != nil {
		if v, ok := frontmatter.Metadata["version"]; ok && v != "" {
			version = v
		}
	}

	// Prepare metadata JSON
	var metadataJSON *string
	if len(frontmatter.Metadata) > 0 {
		b, _ := json.Marshal(frontmatter.Metadata)
		s := string(b)
		metadataJSON = &s
	}

	now := time.Now()
	isDelete := false
	skillType := int32(2) // imported
	status := int32(1)   // enabled

	skill := &model.TSkill{
		CreatedAt:     &now,
		UpdatedAt:     &now,
		IsDelete:      &isDelete,
		Name:          name,
		Slug:          &slug,
		Description:   strPtr(frontmatter.Description),
		SystemPrompt:  body,
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

	// Scan for resource files in scripts/, references/, assets/
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		var category string
		path := filepath.ToSlash(f.Name)
		switch {
		case strings.Contains(path, "scripts/"):
			category = "scripts"
		case strings.Contains(path, "references/"):
			category = "references"
		case strings.Contains(path, "assets/"):
			category = "assets"
		default:
			continue
		}

		r, err := f.Open()
		if err != nil {
			continue // skip files we can't open
		}
		content, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			continue
		}

		contentStr := string(content)
		filename := filepath.Base(f.Name)
		contentType := guessContentType(filename)
		size := int32(len(contentStr))

		res := &model.TSkillResource{
			SkillID:     skill.ID,
			Category:    category,
			Filename:    filename,
			Content:     &contentStr,
			ContentType: &contentType,
			Size:        &size,
		}

		_ = q.TSkillResource.WithContext(l.ctx).Create(res)
	}

	return NewSkillLogic(l.ctx).GetByID(skill.ID)
}

// Export exports a skill to a zip file in Agent Skills standard format
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

	skillMD := generateSkillMD(skill)

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Write SKILL.md
	skillMDPath := filepath.Join(dirName, "SKILL.md")
	w, err := zipWriter.Create(skillMDPath)
	if err != nil {
		return nil, "", fmt.Errorf("create SKILL.md in zip: %w", err)
	}
	if _, err := w.Write([]byte(skillMD)); err != nil {
		return nil, "", err
	}

	// Write resource files
	for _, res := range resources {
		filePath := filepath.Join(dirName, res.Category, res.Filename)
		w, err := zipWriter.Create(filePath)
		if err != nil {
			continue
		}
		content := ""
		if res.Content != nil {
			content = *res.Content
		}
		w.Write([]byte(content))
	}

	if err := zipWriter.Close(); err != nil {
		return nil, "", err
	}

	filename := dirName + ".zip"
	return buf.Bytes(), filename, nil
}

func parseSkillMD(content string) (*SkillMDFrontmatter, string, error) {
	re := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)
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

func generateSkillMD(skill *model.TSkill) string {
	fm := SkillMDFrontmatter{}
	if skill.Slug != nil && *skill.Slug != "" {
		fm.Name = *skill.Slug
	} else {
		fm.Name = sanitizeSlug(skill.Name)
	}
	if skill.Description != nil {
		fm.Description = *skill.Description
	}
	if skill.License != nil && *skill.License != "" {
		fm.License = *skill.License
	}
	if skill.Compatibility != nil && *skill.Compatibility != "" {
		fm.Compatibility = *skill.Compatibility
	}
	if skill.AllowedTools != nil && *skill.AllowedTools != "" {
		fm.AllowedTools = *skill.AllowedTools
	}
	if skill.MetadataJSON != nil && *skill.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(*skill.MetadataJSON), &fm.Metadata)
	}
	if skill.Version != nil && *skill.Version != "" {
		if fm.Metadata == nil {
			fm.Metadata = make(map[string]string)
		}
		fm.Metadata["version"] = *skill.Version
	}

	fmBytes, _ := yaml.Marshal(&fm)
	return fmt.Sprintf("---\n%s---\n\n%s\n", string(fmBytes), skill.SystemPrompt)
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

func guessContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".py":
		return "text/x-python"
	case ".sh", ".bash":
		return "text/x-shellscript"
	case ".js":
		return "text/javascript"
	case ".ts":
		return "text/typescript"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".txt":
		return "text/plain"
	default:
		return "text/plain"
	}
}
