package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/techblog/staticgen/pkg/builder"
	"github.com/techblog/staticgen/pkg/config"
	"github.com/techblog/staticgen/pkg/deploy"
	"github.com/techblog/staticgen/pkg/devserver"
	"github.com/techblog/staticgen/pkg/logger"
)

type CLI struct {
	log *logger.Logger
}

func New() *CLI {
	return &CLI{
		log: logger.New(false),
	}
}

func (c *CLI) Run() error {
	if len(os.Args) < 2 {
		c.printUsage()
		return nil
	}

	cmd := strings.ToLower(os.Args[1])
	args := os.Args[2:]

	switch cmd {
	case "build":
		return c.cmdBuild(args)
	case "dev":
		return c.cmdDev(args)
	case "new":
		return c.cmdNew(args)
	case "deploy":
		return c.cmdDeploy(args)
	case "help", "-h", "--help":
		c.printUsage()
		return nil
	case "version", "-v", "--version":
		c.printVersion()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		c.printUsage()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func (c *CLI) cmdBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	verbose := fs.Bool("v", false, "Verbose output")
	full := fs.Bool("full", false, "Force full rebuild (ignore cache)")
	drafts := fs.Bool("drafts", false, "Include draft posts")
	prettyURLs := fs.Bool("pretty", true, "Use pretty URLs (without .html suffix)")
	noMinify := fs.Bool("no-minify", false, "Disable minification")
	output := fs.String("o", "", "Output directory (default: public)")

	fs.Parse(args)

	c.log.SetVerbose(*verbose)
	c.log.Info("Building site...")

	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(rootDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cfg.Build.Drafts = *drafts || cfg.Build.Drafts
	cfg.Build.PrettyURLs = *prettyURLs
	if *noMinify {
		cfg.Build.MinifyHTML = false
		cfg.Build.MinifyCSS = false
		cfg.Build.MinifyJS = false
	}
	if *output != "" {
		if filepath.IsAbs(*output) {
			cfg.Paths.Public = *output
		} else {
			cfg.Paths.Public = filepath.Join(rootDir, *output)
		}
	}

	b := builder.NewBuilder(cfg, c.log)
	stats, err := b.Build(*full)
	if err != nil {
		return err
	}

	c.log.Success("Build complete!")
	c.log.Info("  Posts:       %d", stats.Posts)
	c.log.Info("  Pages:       %d", stats.Pages)
	c.log.Info("  Static:      %d", stats.StaticFiles)
	c.log.Info("  Output:      %s", cfg.Paths.Public)
	c.log.Info("  Duration:    %v", stats.Duration)

	return nil
}

func (c *CLI) cmdDev(args []string) error {
	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	verbose := fs.Bool("v", false, "Verbose output")
	host := fs.String("host", "", "Host to bind (default: localhost)")
	port := fs.Int("p", 0, "Port to listen (default: 8080)")
	noHotReload := fs.Bool("no-hot", false, "Disable hot reload")
	drafts := fs.Bool("drafts", true, "Include draft posts")
	noBrowser := fs.Bool("no-browser", false, "Don't open browser automatically")

	fs.Parse(args)

	c.log.SetVerbose(*verbose)
	c.log.Info("Starting development server...")

	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(rootDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cfg.Build.Drafts = *drafts
	cfg.Build.Incremental = false
	cfg.Build.MinifyHTML = false
	cfg.Build.MinifyCSS = false
	cfg.Build.MinifyJS = false
	cfg.Build.HashStaticAssets = false
	cfg.Dev.HotReload = !*noHotReload
	cfg.Dev.OpenBrowser = !*noBrowser

	if *host != "" {
		cfg.Dev.Host = *host
	}
	if *port > 0 {
		cfg.Dev.Port = *port
	}

	b := builder.NewBuilder(cfg, c.log)
	c.log.Info("Initial build...")
	if _, err := b.Build(false); err != nil {
		return fmt.Errorf("initial build: %w", err)
	}

	server, err := devserver.New(cfg, c.log, b)
	if err != nil {
		return fmt.Errorf("create dev server: %w", err)
	}

	return server.Start()
}

func (c *CLI) cmdNew(args []string) error {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: staticgen new <post|page> [title]")
		return fmt.Errorf("missing type argument")
	}

	contentType := strings.ToLower(args[0])
	var title string
	if len(args) > 1 {
		title = strings.Join(args[1:], " ")
	}

	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(rootDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	switch contentType {
	case "post", "posts":
		return c.createPost(cfg, title)
	case "page":
		return c.createPage(cfg, title)
	default:
		return fmt.Errorf("unknown content type: %s (use 'post' or 'page')", contentType)
	}
}

func (c *CLI) createPost(cfg *config.Config, title string) error {
	if title == "" {
		title = "New Post"
	}

	now := time.Now()
	date := now.Format("2006-01-02")
	slug := slugify(title)
	filename := fmt.Sprintf("%s-%s.md", date, slug)

	postsDir := filepath.Join(cfg.Paths.Content, "posts")
	if err := os.MkdirAll(postsDir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(postsDir, filename)

	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("file already exists: %s", filePath)
	}

	tmpl := `---
title: "{{.Title}}"
date: {{.Date}}
tags: []
categories: []
draft: true
summary: ""
cover: ""
---

# {{.Title}}

Write your content here...
`

	data := map[string]string{
		"Title": title,
		"Date":  now.Format("2006-01-02"),
	}

	t := template.Must(template.New("post").Parse(tmpl))
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := t.Execute(f, data); err != nil {
		return err
	}

	c.log.Success("Created post: %s", filePath)
	return nil
}

func (c *CLI) createPage(cfg *config.Config, title string) error {
	if title == "" {
		title = "New Page"
	}

	slug := slugify(title)
	filename := fmt.Sprintf("%s.md", slug)

	filePath := filepath.Join(cfg.Paths.Content, filename)

	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("file already exists: %s", filePath)
	}

	tmpl := `---
title: "{{.Title}}"
date: {{.Date}}
summary: ""
---

# {{.Title}}

Write your content here...
`

	data := map[string]string{
		"Title": title,
		"Date":  time.Now().Format("2006-01-02"),
	}

	t := template.Must(template.New("page").Parse(tmpl))
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := t.Execute(f, data); err != nil {
		return err
	}

	c.log.Success("Created page: %s", filePath)
	return nil
}

func (c *CLI) cmdDeploy(args []string) error {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	verbose := fs.Bool("v", false, "Verbose output")
	deployType := fs.String("type", "", "Deploy type: dir|git|rsync|s3")
	target := fs.String("target", "", "Deploy target")
	buildFirst := fs.Bool("build", true, "Build before deploying")

	fs.Parse(args)

	c.log.SetVerbose(*verbose)

	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(rootDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if *deployType != "" {
		cfg.Deploy.Type = *deployType
	}
	if *target != "" {
		cfg.Deploy.Target = *target
	}

	if *buildFirst {
		c.log.Info("Building site before deploy...")
		b := builder.NewBuilder(cfg, c.log)
		if _, err := b.Build(false); err != nil {
			return fmt.Errorf("build: %w", err)
		}
	}

	d := deploy.New(cfg, c.log)
	return d.Deploy()
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ".", "-")
	return s
}

func (c *CLI) printUsage() {
	fmt.Println(`staticgen - Static Site Generator

Usage:
  staticgen <command> [options]

Commands:
  build      Build the static site
  dev        Start development server with hot reload
  new        Create new content
    new post [title]   Create a new blog post
    new page [title]   Create a new page
  deploy     Deploy the site
  version    Show version information
  help       Show this help message

Build Options:
  -v              Verbose output
  -full           Force full rebuild
  -drafts         Include draft posts
  -pretty         Use pretty URLs (default true)
  -no-minify      Disable minification
  -o <dir>        Output directory

Dev Options:
  -v              Verbose output
  -host <host>    Host to bind (default: localhost)
  -p <port>       Port to listen (default: 8080)
  -no-hot         Disable hot reload
  -drafts         Include draft posts (default true)
  -no-browser     Don't open browser

New Options:
  post [title]    Create a new blog post
  page [title]    Create a new page

Deploy Options:
  -v              Verbose output
  -type <type>    Deploy type: dir|git|rsync|s3
  -target <tgt>   Deploy target
  -build=false    Skip build before deploy

Examples:
  staticgen build
  staticgen build -full -drafts
  staticgen dev -p 3000
  staticgen new post "My First Post"
  staticgen new page "About Us"
  staticgen deploy -type dir -target ./dist
`)
}

func (c *CLI) printVersion() {
	fmt.Println("staticgen v1.0.0")
	fmt.Println("A static site generator built with Go")
}
