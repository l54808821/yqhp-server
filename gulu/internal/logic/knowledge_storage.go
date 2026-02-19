package logic

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"yqhp/gulu/internal/svc"
)

var (
	storageInstance *FileStorage
	storageOnce    sync.Once
)

// FileStorage 文件存储服务
type FileStorage struct {
	basePath string
}

// GetFileStorage 获取文件存储单例
func GetFileStorage() *FileStorage {
	storageOnce.Do(func() {
		basePath := "./data/knowledge"
		if svc.Ctx != nil && svc.Ctx.Config != nil && svc.Ctx.Config.Storage.Local.BasePath != "" {
			basePath = svc.Ctx.Config.Storage.Local.BasePath
		}
		if err := os.MkdirAll(basePath, 0755); err != nil {
			panic(fmt.Sprintf("创建存储目录失败: %v", err))
		}
		storageInstance = &FileStorage{basePath: basePath}
	})
	return storageInstance
}

// Save 保存文件，返回相对路径
func (s *FileStorage) Save(kbID int64, filename string, reader io.Reader) (string, error) {
	dir := filepath.Join(s.basePath, fmt.Sprintf("kb_%d", kbID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	ts := time.Now().UnixMilli()
	ext := filepath.Ext(filename)
	storedName := fmt.Sprintf("%d%s", ts, ext)
	relPath := filepath.Join(fmt.Sprintf("kb_%d", kbID), storedName)
	fullPath := filepath.Join(s.basePath, relPath)

	f, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return relPath, nil
}

// Read 读取文件内容
func (s *FileStorage) Read(relPath string) ([]byte, error) {
	fullPath := filepath.Join(s.basePath, relPath)
	return os.ReadFile(fullPath)
}

// Delete 删除文件
func (s *FileStorage) Delete(relPath string) error {
	fullPath := filepath.Join(s.basePath, relPath)
	return os.Remove(fullPath)
}

// DeleteDir 删除知识库目录
func (s *FileStorage) DeleteDir(kbID int64) error {
	dir := filepath.Join(s.basePath, fmt.Sprintf("kb_%d", kbID))
	return os.RemoveAll(dir)
}

// FullPath 返回完整物理路径
func (s *FileStorage) FullPath(relPath string) string {
	return filepath.Join(s.basePath, relPath)
}
