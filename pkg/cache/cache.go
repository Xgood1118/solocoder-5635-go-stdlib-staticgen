package cache

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FileCache struct {
	Path    string    `json:"path"`
	Hash    string    `json:"hash"`
	ModTime time.Time `json:"modTime"`
	Size    int64     `json:"size"`
}

type BuildCache struct {
	Files     map[string]*FileCache `json:"files"`
	BuildTime time.Time             `json:"buildTime"`
	mu        sync.RWMutex          `json:"-"`
	cachePath string                `json:"-"`
}

func Load(cachePath string) (*BuildCache, error) {
	cache := &BuildCache{
		Files:     make(map[string]*FileCache),
		cachePath: cachePath,
	}

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return cache, nil
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return cache, nil
	}

	if err := json.Unmarshal(data, cache); err != nil {
		return cache, nil
	}

	if cache.Files == nil {
		cache.Files = make(map[string]*FileCache)
	}

	return cache, nil
}

func (c *BuildCache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.cachePath == "" {
		return nil
	}

	c.BuildTime = time.Now()

	dir := filepath.Dir(c.cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.cachePath, data, 0644)
}

func (c *BuildCache) IsChanged(path string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if _, exists := c.Files[path]; exists {
				return true, nil
			}
			return false, nil
		}
		return false, err
	}

	existing, exists := c.Files[path]
	if !exists {
		return true, nil
	}

	if info.Size() != existing.Size {
		return true, nil
	}

	if !info.ModTime().Equal(existing.ModTime) {
		return true, nil
	}

	hash, err := computeHash(path)
	if err != nil {
		return true, nil
	}

	if hash != existing.Hash {
		return true, nil
	}

	return false, nil
}

func (c *BuildCache) Update(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	hash, err := computeHash(path)
	if err != nil {
		return err
	}

	c.Files[path] = &FileCache{
		Path:    path,
		Hash:    hash,
		ModTime: info.ModTime(),
		Size:    info.Size(),
	}

	return nil
}

func (c *BuildCache) Remove(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Files, path)
}

func (c *BuildCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Files = make(map[string]*FileCache)
}

func computeHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:]), nil
}

func ComputeHashBytes(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}
