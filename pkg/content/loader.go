package content

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/techblog/staticgen/pkg/config"
	"github.com/techblog/staticgen/pkg/frontmatter"
	"github.com/techblog/staticgen/pkg/logger"
	"github.com/techblog/staticgen/pkg/markdown"
	"github.com/techblog/staticgen/pkg/models"
)

type Loader struct {
	cfg       *config.Config
	log       *logger.Logger
	converter *markdown.Converter
}

func NewLoader(cfg *config.Config, log *logger.Logger) *Loader {
	return &Loader{
		cfg:       cfg,
		log:       log,
		converter: markdown.NewConverter(cfg.Paths.Content, cfg.Build.PrettyURLs),
	}
}

func (l *Loader) LoadAll() ([]*models.Post, []*models.Page, error) {
	var posts []*models.Post
	var pages []*models.Page

	if _, err := os.Stat(l.cfg.Paths.Content); os.IsNotExist(err) {
		l.log.Warn("Content directory not found: %s", l.cfg.Paths.Content)
		return posts, pages, nil
	}

	err := filepath.WalkDir(l.cfg.Paths.Content, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}

		relPath, err := filepath.Rel(l.cfg.Paths.Content, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if strings.ToLower(filepath.Base(relPath)) == "about.md" {
			page, err := l.loadAboutPage(path, relPath)
			if err != nil {
				l.log.Error("Failed to load about page %s: %v", relPath, err)
				return nil
			}
			pages = append(pages, page)
			return nil
		}

		if strings.Contains(relPath, string(filepath.Separator)) || strings.Contains(relPath, "/") {
			firstDir := strings.Split(relPath, "/")[0]
			if strings.ToLower(firstDir) == "posts" {
				post, err := l.loadPost(path, relPath)
				if err != nil {
					l.log.Error("Failed to load post %s: %v", relPath, err)
					return nil
				}
				if post != nil {
					posts = append(posts, post)
				}
				return nil
			}
		}

		post, err := l.loadPost(path, relPath)
		if err != nil {
			l.log.Error("Failed to load content %s: %v", relPath, err)
			return nil
		}
		if post != nil {
			posts = append(posts, post)
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	for i := range posts {
		if i > 0 {
			posts[i].Next = posts[i-1]
		}
		if i < len(posts)-1 {
			posts[i].Previous = posts[i+1]
		}
	}

	return posts, pages, nil
}

func (l *Loader) loadPost(path, relPath string) (*models.Post, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result, err := frontmatter.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	fm := result.FrontMatter
	if fm.Draft && !l.cfg.Build.Drafts {
		l.log.Debug("Skipping draft: %s", relPath)
		return nil, nil
	}

	date, err := frontmatter.ParseDate(fm.Date)
	if err != nil {
		l.log.Warn("Failed to parse date for %s, using now", relPath)
	}

	slug := extractSlug(filepath.Base(relPath))
	if fm.Title == "" {
		fm.Title = slug
	}

	html, err := l.converter.Convert([]byte(result.Content), relPath)
	if err != nil {
		return nil, fmt.Errorf("convert markdown: %w", err)
	}

	url := l.buildPostURL(relPath, slug)
	wordCount := markdown.CountWords(result.Content)

	post := &models.Post{
		Title:       fm.Title,
		Slug:        slug,
		Date:        date,
		Tags:        fm.Tags,
		Categories:  fm.Categories,
		Draft:       fm.Draft,
		Summary:     fm.Summary,
		Cover:       fm.Cover,
		Author:      fm.Author,
		Content:     result.Content,
		HTML:        html,
		Path:        relPath,
		URL:         url,
		FileName:    filepath.Base(path),
		WordCount:   wordCount,
		ReadingTime: markdown.EstimateReadingTime(wordCount),
	}

	if post.Summary == "" {
		post.Summary = extractSummary(result.Content)
	}

	return post, nil
}

func (l *Loader) loadAboutPage(path, relPath string) (*models.Page, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result, err := frontmatter.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	fm := result.FrontMatter
	if fm.Title == "" {
		fm.Title = "关于"
	}

	html, err := l.converter.Convert([]byte(result.Content), relPath)
	if err != nil {
		return nil, fmt.Errorf("convert markdown: %w", err)
	}

	var url string
	if l.cfg.Build.PrettyURLs {
		url = "/about/"
	} else {
		url = "/about.html"
	}

	return &models.Page{
		Title:   fm.Title,
		Content: result.Content,
		HTML:    html,
		Path:    relPath,
		URL:     url,
		Data: map[string]interface{}{
			"date":    fm.Date,
			"summary": fm.Summary,
		},
	}, nil
}

func (l *Loader) buildPostURL(relPath, slug string) string {
	relPath = filepath.ToSlash(relPath)
	dir := filepath.Dir(relPath)
	dir = strings.TrimSuffix(dir, ".")

	if dir == "posts" || dir == "." || dir == "" {
		if l.cfg.Build.PrettyURLs {
			return "/posts/" + slug + "/"
		}
		return "/posts/" + slug + ".html"
	}

	cleanDir := regexp.MustCompile(`^posts/`).ReplaceAllString(dir, "")
	if cleanDir == "" {
		if l.cfg.Build.PrettyURLs {
			return "/posts/" + slug + "/"
		}
		return "/posts/" + slug + ".html"
	}

	if l.cfg.Build.PrettyURLs {
		return "/" + cleanDir + "/" + slug + "/"
	}
	return "/" + cleanDir + "/" + slug + ".html"
}

func extractSlug(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}-`, name); matched {
		name = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-`).ReplaceAllString(name, "")
	}
	return name
}

func extractSummary(content string) string {
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")
	var summaryLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(summaryLines) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		summaryLines = append(summaryLines, line)
		if len(summaryLines) >= 3 {
			break
		}
	}
	summary := strings.Join(summaryLines, " ")
	runes := []rune(summary)
	if len(runes) > 200 {
		summary = string(runes[:200]) + "..."
	}
	return summary
}

func BuildSiteData(cfg *config.Config, posts []*models.Post, pages []*models.Page) *models.Site {
	categories := make(map[string]*models.Category)
	tags := make(map[string]*models.Tag)

	for _, post := range posts {
		for _, catName := range post.Categories {
			if _, ok := categories[catName]; !ok {
				var catURL string
				if cfg.Build.PrettyURLs {
					catURL = "/categories/" + urlize(catName) + "/"
				} else {
					catURL = "/categories/" + urlize(catName) + ".html"
				}
				categories[catName] = &models.Category{
					Name:  catName,
					URL:   catURL,
					Posts: []*models.Post{},
				}
			}
			categories[catName].Posts = append(categories[catName].Posts, post)
			categories[catName].Count++
		}

		for _, tagName := range post.Tags {
			if _, ok := tags[tagName]; !ok {
				var tagURL string
				if cfg.Build.PrettyURLs {
					tagURL = "/tags/" + urlize(tagName) + "/"
				} else {
					tagURL = "/tags/" + urlize(tagName) + ".html"
				}
				tags[tagName] = &models.Tag{
					Name:  tagName,
					URL:   tagURL,
					Posts: []*models.Post{},
				}
			}
			tags[tagName].Posts = append(tags[tagName].Posts, post)
			tags[tagName].Count++
		}
	}

	var allCategories []*models.Category
	for _, cat := range categories {
		allCategories = append(allCategories, cat)
	}
	sort.Slice(allCategories, func(i, j int) bool {
		return allCategories[i].Name < allCategories[j].Name
	})

	var allTags []*models.Tag
	for _, tag := range tags {
		allTags = append(allTags, tag)
	}
	sort.Slice(allTags, func(i, j int) bool {
		return allTags[i].Name < allTags[j].Name
	})

	recentPosts := posts
	if len(posts) > cfg.Pagination.PageSize {
		recentPosts = posts[:cfg.Pagination.PageSize]
	}

	return &models.Site{
		Title:         cfg.Site.Title,
		Description:   cfg.Site.Description,
		Author:        cfg.Site.Author,
		Language:      cfg.Site.Language,
		BaseURL:       cfg.Site.BaseURL,
		URL:           cfg.Site.URL,
		Copyright:     cfg.Site.Copyright,
		Keywords:      cfg.Site.Keywords,
		Posts:         posts,
		Pages:         pages,
		Categories:    categories,
		Tags:          tags,
		RecentPosts:   recentPosts,
		AllTags:       allTags,
		AllCategories: allCategories,
	}
}

func urlize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}
