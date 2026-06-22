package template

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/techblog/staticgen/pkg/models"
)

type Engine struct {
	templates    *template.Template
	templateDir  string
	staticAssets map[string]string
	funcMap      template.FuncMap
}

func NewEngine(templateDir string, staticAssets map[string]string) (*Engine, error) {
	engine := &Engine{
		templateDir:  templateDir,
		staticAssets: staticAssets,
	}
	engine.funcMap = engine.buildFuncMap()

	if err := engine.LoadTemplates(); err != nil {
		return nil, err
	}

	return engine, nil
}

func (e *Engine) buildFuncMap() template.FuncMap {
	return template.FuncMap{
		"date": func(format string, t time.Time) string {
			return t.Format(format)
		},
		"dateFormat": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
		"dateTimeFormat": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"safeCSS": func(s string) template.CSS {
			return template.CSS(s)
		},
		"safeJS": func(s string) template.JS {
			return template.JS(s)
		},
		"safeURL": func(s string) template.URL {
			return template.URL(s)
		},
		"truncate": func(s string, n int) string {
			runes := []rune(s)
			if len(runes) <= n {
				return s
			}
			return string(runes[:n]) + "..."
		},
		"join": func(sep string, items []string) string {
			return strings.Join(items, sep)
		},
		"contains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
		"replace": func(s, old, new string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
		"trim": func(s string) string {
			return strings.TrimSpace(s)
		},
		"split": func(sep, s string) []string {
			return strings.Split(s, sep)
		},
		"len": func(v interface{}) int {
			switch val := v.(type) {
			case []*models.Post:
				return len(val)
			case []string:
				return len(val)
			case []*models.Category:
				return len(val)
			case []*models.Tag:
				return len(val)
			case string:
				return len(val)
			default:
				return 0
			}
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"mul": func(a, b int) int {
			return a * b
		},
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mod": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a % b
		},
		"asset": func(path string) string {
			if e.staticAssets != nil {
				if hashed, ok := e.staticAssets[path]; ok {
					return hashed
				}
			}
			return path
		},
		"jsonify": func(v interface{}) string {
			return fmt.Sprintf("%v", v)
		},
		"default": func(def, v interface{}) interface{} {
			if v == nil {
				return def
			}
			if s, ok := v.(string); ok && s == "" {
				return def
			}
			return v
		},
		"first": func(posts []*models.Post, n int) []*models.Post {
			if len(posts) <= n {
				return posts
			}
			return posts[:n]
		},
		"firstStrings": func(items []string, n int) []string {
			if len(items) <= n {
				return items
			}
			return items[:n]
		},
		"last": func(posts []*models.Post, n int) []*models.Post {
			if len(posts) <= n {
				return posts
			}
			return posts[len(posts)-n:]
		},
		"slice": func(start, end int, items []*models.Post) []*models.Post {
			if start < 0 {
				start = 0
			}
			if end > len(items) {
				end = len(items)
			}
			if start >= end {
				return []*models.Post{}
			}
			return items[start:end]
		},
		"makeRange": func(n int) []int {
			result := make([]int, n)
			for i := range result {
				result[i] = i + 1
			}
			return result
		},
		"now": func() time.Time {
			return time.Now()
		},
		"year": func(t time.Time) int {
			return t.Year()
		},
		"month": func(t time.Time) int {
			return int(t.Month())
		},
		"day": func(t time.Time) int {
			return t.Day()
		},
		"urlize": func(s string) string {
			s = strings.ToLower(s)
			s = strings.ReplaceAll(s, " ", "-")
			s = strings.ReplaceAll(s, "_", "-")
			return s
		},
	}
}

func (e *Engine) LoadTemplates() error {
	if _, err := os.Stat(e.templateDir); os.IsNotExist(err) {
		return fmt.Errorf("template directory not found: %s", e.templateDir)
	}

	tmpl := template.New("").Funcs(e.funcMap)

	err := filepath.WalkDir(e.templateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".html") {
			return nil
		}

		relPath, err := filepath.Rel(e.templateDir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relPath)
		name = strings.TrimSuffix(name, ".html")

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", name, err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	e.templates = tmpl
	return nil
}

func (e *Engine) Render(name string, data interface{}) (string, error) {
	if e.templates == nil {
		return "", fmt.Errorf("templates not loaded")
	}

	t := e.templates.Lookup(name)
	if t == nil {
		return "", fmt.Errorf("template not found: %s", name)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template %s: %w", name, err)
	}

	return buf.String(), nil
}

func (e *Engine) RenderWithBase(baseName, contentName string, data *models.TemplateContext) (string, error) {
	content, err := e.Render(contentName, data)
	if err != nil {
		return "", err
	}

	var pageTitle string
	if data.IsPost && data.Post != nil {
		pageTitle = data.Post.Title
	} else if data.IsPage {
		if p, ok := data.Page.(*models.Page); ok && p != nil {
			pageTitle = p.Title
		}
	} else if data.IsCategory && data.Category != nil {
		pageTitle = "分类: " + data.Category.Name
	} else if data.IsTag && data.Tag != nil {
		pageTitle = "标签: #" + data.Tag.Name
	} else if data.IsArchive {
		pageTitle = "文章归档"
	} else if data.Is404 {
		pageTitle = "页面未找到"
	} else if data.IsHome {
		pageTitle = ""
	}

	baseData := map[string]interface{}{
		"Site":         data.Site,
		"Content":      template.HTML(content),
		"CurrentURL":   data.CurrentURL,
		"StaticAssets": data.StaticAssets,
		"Post":         data.Post,
		"PageData":     data.Page,
		"PageTitle":    pageTitle,
		"IsHome":       data.IsHome,
		"IsArchive":    data.IsArchive,
		"IsCategory":   data.IsCategory,
		"IsTag":        data.IsTag,
		"IsPage":       data.IsPage,
		"IsPost":       data.IsPost,
		"Is404":        data.Is404,
	}

	return e.Render(baseName, baseData)
}

func (e *Engine) Reload() error {
	return e.LoadTemplates()
}

func (e *Engine) HasTemplate(name string) bool {
	if e.templates == nil {
		return false
	}
	return e.templates.Lookup(name) != nil
}
