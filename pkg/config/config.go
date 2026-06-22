package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Site      SiteConfig      `yaml:"site"`
	Build     BuildConfig     `yaml:"build"`
	Dev       DevConfig       `yaml:"dev"`
	Deploy    DeployConfig    `yaml:"deploy"`
	Theme     ThemeConfig     `yaml:"theme"`
	Pagination PaginationConfig `yaml:"pagination"`
	Paths     Paths           `yaml:"-"`
}

type SiteConfig struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Author      string   `yaml:"author"`
	Language    string   `yaml:"language"`
	BaseURL     string   `yaml:"baseURL"`
	URL         string   `yaml:"url"`
	Copyright   string   `yaml:"copyright"`
	Keywords    []string `yaml:"keywords"`
}

type BuildConfig struct {
	PrettyURLs       bool `yaml:"prettyURLs"`
	MinifyHTML       bool `yaml:"minifyHTML"`
	MinifyCSS        bool `yaml:"minifyCSS"`
	MinifyJS         bool `yaml:"minifyJS"`
	Incremental      bool `yaml:"incremental"`
	HashStaticAssets bool `yaml:"hashStaticAssets"`
	Drafts           bool `yaml:"drafts"`
	Workers          int  `yaml:"workers"`
}

type DevConfig struct {
	Port    int  `yaml:"port"`
	Host    string `yaml:"host"`
	HotReload bool `yaml:"hotReload"`
	OpenBrowser bool `yaml:"openBrowser"`
}

type DeployConfig struct {
	Type   string            `yaml:"type"`
	Target string            `yaml:"target"`
	Options map[string]string `yaml:"options"`
}

type ThemeConfig struct {
	Name string `yaml:"name"`
	Dir  string `yaml:"dir"`
}

type PaginationConfig struct {
	PageSize int `yaml:"pageSize"`
}

type Paths struct {
	Root       string
	Content    string
	Static     string
	Public     string
	Themes     string
	CurrentTheme string
	Templates  string
	CacheFile  string
}

func Default() *Config {
	return &Config{
		Site: SiteConfig{
			Title:       "技术博客",
			Description: "一个使用 Go 构建的技术博客",
			Author:      "Admin",
			Language:    "zh-CN",
			BaseURL:     "/",
			URL:         "http://localhost:8080",
			Copyright:   "© 2024 All Rights Reserved",
		},
		Build: BuildConfig{
			PrettyURLs:       true,
			MinifyHTML:       true,
			MinifyCSS:        true,
			MinifyJS:         true,
			Incremental:      true,
			HashStaticAssets: true,
			Drafts:           false,
			Workers:          4,
		},
		Dev: DevConfig{
			Port:       8080,
			Host:       "localhost",
			HotReload:  true,
			OpenBrowser: false,
		},
		Deploy: DeployConfig{
			Type:   "dir",
			Target: "./dist",
		},
		Theme: ThemeConfig{
			Name: "default",
			Dir:  "themes",
		},
		Pagination: PaginationConfig{
			PageSize: 10,
		},
	}
}

func Load(rootDir string) (*Config, error) {
	cfg := Default()
	cfg.initPaths(rootDir)

	configPath := filepath.Join(rootDir, "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
		cfg.initPaths(rootDir)
	}

	return cfg, nil
}

func (c *Config) initPaths(rootDir string) {
	c.Paths.Root = rootDir
	c.Paths.Content = filepath.Join(rootDir, "content")
	c.Paths.Static = filepath.Join(rootDir, "static")
	c.Paths.Public = filepath.Join(rootDir, "public")
	c.Paths.Themes = filepath.Join(rootDir, c.Theme.Dir)
	c.Paths.CurrentTheme = filepath.Join(c.Paths.Themes, c.Theme.Name)
	c.Paths.Templates = filepath.Join(c.Paths.CurrentTheme, "templates")
	c.Paths.CacheFile = filepath.Join(rootDir, ".build-cache.json")
}
