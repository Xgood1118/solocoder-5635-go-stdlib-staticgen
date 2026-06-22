package assets

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/js"
)

type Manager struct {
	sourceDirs   []string
	outputDir    string
	useHash      bool
	minifyCSS    bool
	minifyJS     bool
	assetMap     map[string]string
	minifier     *minify.M
}

func NewManager(outputDir string, useHash, minifyCSS, minifyJS bool) *Manager {
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("application/javascript", js.Minify)

	return &Manager{
		outputDir: outputDir,
		useHash:   useHash,
		minifyCSS: minifyCSS,
		minifyJS:  minifyJS,
		assetMap:  make(map[string]string),
		minifier:  m,
	}
}

func (m *Manager) AddSourceDir(dir string) {
	m.sourceDirs = append(m.sourceDirs, dir)
}

func (m *Manager) GetAssetMap() map[string]string {
	return m.assetMap
}

func (m *Manager) Process() error {
	for _, sourceDir := range m.sourceDirs {
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			continue
		}

		err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(sourceDir, path)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath)

			return m.processFile(path, relPath)
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) processFile(srcPath, relPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(relPath))
	data, err = m.maybeMinify(data, ext)
	if err != nil {
		return err
	}

	outputRelPath := relPath
	if m.useHash && shouldHash(ext) {
		hash := computeShortHash(data)
		outputRelPath = insertHashBeforeExt(relPath, hash)
	}

	dstPath := filepath.Join(m.outputDir, filepath.FromSlash(outputRelPath))

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return err
	}

	originalURL := "/" + relPath
	hashedURL := "/" + outputRelPath
	m.assetMap[originalURL] = hashedURL

	return nil
}

func (m *Manager) maybeMinify(data []byte, ext string) ([]byte, error) {
	switch ext {
	case ".css":
		if !m.minifyCSS {
			return data, nil
		}
		return m.minifier.Bytes("text/css", data)
	case ".js":
		if !m.minifyJS {
			return data, nil
		}
		return m.minifier.Bytes("application/javascript", data)
	default:
		return data, nil
	}
}

func (m *Manager) CopyFile(srcPath, relPath string) (string, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(relPath))
	data, err = m.maybeMinify(data, ext)
	if err != nil {
		return "", err
	}

	outputRelPath := relPath
	if m.useHash && shouldHash(ext) {
		hash := computeShortHash(data)
		outputRelPath = insertHashBeforeExt(relPath, hash)
	}

	dstPath := filepath.Join(m.outputDir, filepath.FromSlash(outputRelPath))
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return "", err
	}

	originalURL := "/" + relPath
	hashedURL := "/" + outputRelPath
	m.assetMap[originalURL] = hashedURL

	return hashedURL, nil
}

func (m *Manager) CopyDir(srcDir, dstRelDir string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		fullRelPath := filepath.ToSlash(filepath.Join(dstRelDir, relPath))

		_, err = m.CopyFile(path, fullRelPath)
		return err
	})
}

func (m *Manager) CopyReader(r io.Reader, relPath string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(relPath))
	data, err = m.maybeMinify(data, ext)
	if err != nil {
		return "", err
	}

	outputRelPath := relPath
	if m.useHash && shouldHash(ext) {
		hash := computeShortHash(data)
		outputRelPath = insertHashBeforeExt(relPath, hash)
	}

	dstPath := filepath.Join(m.outputDir, filepath.FromSlash(outputRelPath))
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return "", err
	}

	originalURL := "/" + relPath
	hashedURL := "/" + outputRelPath
	m.assetMap[originalURL] = hashedURL

	return hashedURL, nil
}

func shouldHash(ext string) bool {
	switch ext {
	case ".css", ".js", ".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".woff", ".woff2", ".ttf", ".eot":
		return true
	default:
		return false
	}
}

func computeShortHash(data []byte) string {
	h := md5.Sum(data)
	return hex.EncodeToString(h[:])[:12]
}

func insertHashBeforeExt(path, hash string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return fmt.Sprintf("%s.%s%s", base, hash, ext)
}
