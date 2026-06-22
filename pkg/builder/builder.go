package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/html"

	"github.com/techblog/staticgen/pkg/assets"
	"github.com/techblog/staticgen/pkg/cache"
	"github.com/techblog/staticgen/pkg/config"
	"github.com/techblog/staticgen/pkg/content"
	"github.com/techblog/staticgen/pkg/logger"
	"github.com/techblog/staticgen/pkg/models"
	templatepkg "github.com/techblog/staticgen/pkg/template"
)

type Builder struct {
	cfg        *config.Config
	log        *logger.Logger
	cache      *cache.BuildCache
	tmplEngine *templatepkg.Engine
	assetMgr   *assets.Manager
	site       *models.Site
	posts      []*models.Post
	pages      []*models.Page
	minifier   *minify.M
}

type BuildStats struct {
	Posts       int
	Pages       int
	StaticFiles int
	Duration    time.Duration
}

func NewBuilder(cfg *config.Config, log *logger.Logger) *Builder {
	m := minify.New()
	m.AddFunc("text/html", html.Minify)

	return &Builder{
		cfg:      cfg,
		log:      log,
		minifier: m,
	}
}

func (b *Builder) Build(fullBuild bool) (*BuildStats, error) {
	start := time.Now()
	b.log.Info("Starting build...")

	if err := os.RemoveAll(b.cfg.Paths.Public); err != nil {
		return nil, fmt.Errorf("clean public dir: %w", err)
	}

	if err := os.MkdirAll(b.cfg.Paths.Public, 0755); err != nil {
		return nil, fmt.Errorf("create public dir: %w", err)
	}

	var err error
	b.cache, err = cache.Load(b.cfg.Paths.CacheFile)
	if err != nil {
		b.log.Warn("Failed to load build cache: %v", err)
		b.cache = &cache.BuildCache{}
	}

	if fullBuild {
		b.cache.Clear()
		b.cfg.Build.Incremental = false
	}

	b.log.Info("Processing static assets...")
	b.assetMgr = assets.NewManager(
		b.cfg.Paths.Public,
		b.cfg.Build.HashStaticAssets,
		b.cfg.Build.MinifyCSS,
		b.cfg.Build.MinifyJS,
	)
	b.assetMgr.AddSourceDir(b.cfg.Paths.Static)

	themeStaticDir := filepath.Join(b.cfg.Paths.CurrentTheme, "static")
	if _, err := os.Stat(themeStaticDir); err == nil {
		b.assetMgr.AddSourceDir(themeStaticDir)
	}

	if err := b.assetMgr.Process(); err != nil {
		return nil, fmt.Errorf("process assets: %w", err)
	}

	b.log.Info("Loading templates...")
	b.tmplEngine, err = templatepkg.NewEngine(b.cfg.Paths.Templates, b.assetMgr.GetAssetMap())
	if err != nil {
		return nil, fmt.Errorf("load templates: %w", err)
	}

	b.log.Info("Loading content...")
	loader := content.NewLoader(b.cfg, b.log)
	b.posts, b.pages, err = loader.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("load content: %w", err)
	}
	b.log.Info("Loaded %d posts, %d pages", len(b.posts), len(b.pages))

	b.site = content.BuildSiteData(b.cfg, b.posts, b.pages)
	b.site.BuildTime = time.Now()

	staticFileCount := len(b.assetMgr.GetAssetMap())

	if err := b.generateAll(); err != nil {
		return nil, err
	}

	if err := b.cache.Save(); err != nil {
		b.log.Warn("Failed to save build cache: %v", err)
	}

	duration := time.Since(start)
	stats := &BuildStats{
		Posts:       len(b.posts),
		Pages:       len(b.pages),
		StaticFiles: staticFileCount,
		Duration:    duration,
	}

	b.log.Success("Build completed: %d posts, %d pages, %d static files in %v",
		stats.Posts, stats.Pages, stats.StaticFiles, duration)

	return stats, nil
}

func (b *Builder) generateAll() error {
	if err := b.generatePosts(); err != nil {
		return err
	}

	if err := b.generatePages(); err != nil {
		return err
	}

	if err := b.generateHome(); err != nil {
		return err
	}

	if err := b.generateArchive(); err != nil {
		return err
	}

	if err := b.generateCategories(); err != nil {
		return err
	}

	if err := b.generateTags(); err != nil {
		return err
	}

	if err := b.generate404(); err != nil {
		return err
	}

	if err := b.generateSitemap(); err != nil {
		return err
	}

	if err := b.generateRSS(); err != nil {
		return err
	}

	if err := b.generateRobots(); err != nil {
		return err
	}

	return nil
}

func (b *Builder) generatePosts() error {
	b.log.Info("Generating %d posts...", len(b.posts))

	workers := b.cfg.Build.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(b.posts) && len(b.posts) > 0 {
		workers = len(b.posts)
	}
	if workers <= 0 {
		workers = 1
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	errChan := make(chan error, len(b.posts))

	for _, post := range b.posts {
		wg.Add(1)
		sem <- struct{}{}

		go func(p *models.Post) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := b.generatePost(p); err != nil {
				errChan <- err
			}
		}(post)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Builder) generatePost(post *models.Post) error {
	ctx := &models.TemplateContext{
		Site:         b.site,
		Post:         post,
		CurrentURL:   post.URL,
		IsPost:       true,
		StaticAssets: b.assetMgr.GetAssetMap(),
	}

	var html string
	var err error

	if b.tmplEngine.HasTemplate("single") {
		html, err = b.tmplEngine.RenderWithBase("base", "single", ctx)
	} else {
		html, err = b.tmplEngine.Render("single", ctx)
	}
	if err != nil {
		return fmt.Errorf("render post %s: %w", post.Slug, err)
	}

	html, err = b.maybeMinifyHTML(html)
	if err != nil {
		return err
	}

	outputPath := b.urlToFilePath(post.URL)
	if err := b.writeFile(outputPath, html); err != nil {
		return err
	}

	b.log.Debug("Generated post: %s", post.URL)
	return nil
}

func (b *Builder) generatePages() error {
	for _, page := range b.pages {
		if err := b.generatePage(page); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) generatePage(page *models.Page) error {
	ctx := &models.TemplateContext{
		Site:         b.site,
		Page:         page,
		CurrentURL:   page.URL,
		IsPage:       true,
		Data:         page.Data,
		StaticAssets: b.assetMgr.GetAssetMap(),
	}

	var html string
	var err error

	if b.tmplEngine.HasTemplate("page") {
		html, err = b.tmplEngine.RenderWithBase("base", "page", ctx)
	} else {
		html, err = b.tmplEngine.Render("page", ctx)
	}
	if err != nil {
		return fmt.Errorf("render page %s: %w", page.URL, err)
	}

	html, err = b.maybeMinifyHTML(html)
	if err != nil {
		return err
	}

	outputPath := b.urlToFilePath(page.URL)
	if err := b.writeFile(outputPath, html); err != nil {
		return err
	}

	b.log.Debug("Generated page: %s", page.URL)
	return nil
}

func (b *Builder) maybeMinifyHTML(s string) (string, error) {
	if !b.cfg.Build.MinifyHTML {
		return s, nil
	}
	return b.minifier.String("text/html", s)
}

func (b *Builder) urlToFilePath(url string) string {
	if url == "/" {
		if b.cfg.Build.PrettyURLs {
			return filepath.Join(b.cfg.Paths.Public, "index.html")
		}
		return filepath.Join(b.cfg.Paths.Public, "index.html")
	}

	url = filepath.FromSlash(url)
	url = filepath.Clean(url)

	if b.cfg.Build.PrettyURLs {
		return filepath.Join(b.cfg.Paths.Public, url, "index.html")
	}
	if filepath.Ext(url) == "" {
		url += ".html"
	}
	return filepath.Join(b.cfg.Paths.Public, url)
}

func (b *Builder) writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func (b *Builder) ReloadTemplates() error {
	return b.tmplEngine.Reload()
}

func (b *Builder) GetTemplateEngine() *templatepkg.Engine {
	return b.tmplEngine
}

func (b *Builder) GetAssetManager() *assets.Manager {
	return b.assetMgr
}

func (b *Builder) GetSite() *models.Site {
	return b.site
}

func (b *Builder) GetPosts() []*models.Post {
	return b.posts
}

func (b *Builder) GetPages() []*models.Page {
	return b.pages
}
