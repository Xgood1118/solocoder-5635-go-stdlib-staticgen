package builder

import (
	"encoding/xml"
	"fmt"
	"html"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/techblog/staticgen/pkg/models"
)

func (b *Builder) generateHome() error {
	b.log.Info("Generating home page...")

	totalPosts := len(b.posts)
	pageSize := b.cfg.Pagination.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	totalPages := int(math.Ceil(float64(totalPosts) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		start := (pageNum - 1) * pageSize
		end := start + pageSize
		if end > totalPosts {
			end = totalPosts
		}

		pagePosts := b.posts[start:end]

		pagination := &models.Pagination{
			Page:       pageNum,
			TotalPages: totalPages,
			TotalItems: totalPosts,
			PageSize:   pageSize,
			Items:      pagePosts,
		}

		if pageNum > 1 {
			pagination.HasPrev = true
			if pageNum == 2 {
				pagination.PrevURL = "/"
			} else {
				pagination.PrevURL = fmt.Sprintf("/page/%d/", pageNum-1)
				if !b.cfg.Build.PrettyURLs {
					pagination.PrevURL = fmt.Sprintf("/page/%d.html", pageNum-1)
				}
			}
		}
		if pageNum < totalPages {
			pagination.HasNext = true
			pagination.NextURL = fmt.Sprintf("/page/%d/", pageNum+1)
			if !b.cfg.Build.PrettyURLs {
				pagination.NextURL = fmt.Sprintf("/page/%d.html", pageNum+1)
			}
		}
		pagination.FirstURL = "/"
		if totalPages > 1 {
			pagination.LastURL = fmt.Sprintf("/page/%d/", totalPages)
			if !b.cfg.Build.PrettyURLs {
				pagination.LastURL = fmt.Sprintf("/page/%d.html", totalPages)
			}
		} else {
			pagination.LastURL = "/"
		}

		var currentURL string
		if pageNum == 1 {
			currentURL = "/"
		} else {
			currentURL = fmt.Sprintf("/page/%d/", pageNum)
			if !b.cfg.Build.PrettyURLs {
				currentURL = fmt.Sprintf("/page/%d.html", pageNum)
			}
		}
		pagination.CurrentURL = currentURL

		ctx := &models.TemplateContext{
			Site:         b.site,
			Posts:        pagePosts,
			Pagination:   pagination,
			CurrentURL:   currentURL,
			IsHome:       pageNum == 1,
			StaticAssets: b.assetMgr.GetAssetMap(),
		}

		var html string
		var err error

		if b.tmplEngine.HasTemplate("list") {
			html, err = b.tmplEngine.RenderWithBase("base", "list", ctx)
		} else {
			html, err = b.tmplEngine.Render("list", ctx)
		}
		if err != nil {
			return fmt.Errorf("render home page %d: %w", pageNum, err)
		}

		html, err = b.maybeMinifyHTML(html)
		if err != nil {
			return err
		}

		outputPath := b.urlToFilePath(currentURL)
		if err := b.writeFile(outputPath, html); err != nil {
			return err
		}

		b.log.Debug("Generated home page: %s", currentURL)
	}

	return nil
}

func (b *Builder) generateArchive() error {
	b.log.Info("Generating archive page...")

	type YearGroup struct {
		Year  int
		Posts []*models.Post
	}

	yearMap := make(map[int][]*models.Post)
	for _, post := range b.posts {
		year := post.Date.Year()
		yearMap[year] = append(yearMap[year], post)
	}

	var years []int
	for y := range yearMap {
		years = append(years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	var groups []*YearGroup
	for _, y := range years {
		posts := yearMap[y]
		sort.Slice(posts, func(i, j int) bool {
			return posts[i].Date.After(posts[j].Date)
		})
		groups = append(groups, &YearGroup{Year: y, Posts: posts})
	}

	var url string
	if b.cfg.Build.PrettyURLs {
		url = "/archive/"
	} else {
		url = "/archive.html"
	}

	ctx := &models.TemplateContext{
		Site:         b.site,
		Posts:        b.posts,
		CurrentURL:   url,
		IsArchive:    true,
		Data: map[string]interface{}{
			"YearGroups": groups,
		},
		StaticAssets: b.assetMgr.GetAssetMap(),
	}

	var html string
	var err error

	if b.tmplEngine.HasTemplate("archive") {
		html, err = b.tmplEngine.RenderWithBase("base", "archive", ctx)
	} else if b.tmplEngine.HasTemplate("list") {
		html, err = b.tmplEngine.RenderWithBase("base", "list", ctx)
	} else {
		html, err = b.tmplEngine.Render("list", ctx)
	}
	if err != nil {
		return fmt.Errorf("render archive: %w", err)
	}

	html, err = b.maybeMinifyHTML(html)
	if err != nil {
		return err
	}

	outputPath := b.urlToFilePath(url)
	if err := b.writeFile(outputPath, html); err != nil {
		return err
	}

	b.log.Debug("Generated archive: %s", url)
	return nil
}

func (b *Builder) generateCategories() error {
	b.log.Info("Generating category pages...")

	var catListURL string
	if b.cfg.Build.PrettyURLs {
		catListURL = "/categories/"
	} else {
		catListURL = "/categories.html"
	}

	ctx := &models.TemplateContext{
		Site:         b.site,
		CurrentURL:   catListURL,
		IsCategory:   true,
		StaticAssets: b.assetMgr.GetAssetMap(),
	}

	var html string
	var err error

	if b.tmplEngine.HasTemplate("categories") {
		html, err = b.tmplEngine.RenderWithBase("base", "categories", ctx)
	} else if b.tmplEngine.HasTemplate("list") {
		html, err = b.tmplEngine.RenderWithBase("base", "list", ctx)
	} else {
		html, err = b.tmplEngine.Render("list", ctx)
	}
	if err != nil {
		return fmt.Errorf("render categories list: %w", err)
	}
	html, err = b.maybeMinifyHTML(html)
	if err != nil {
		return err
	}
	outputPath := b.urlToFilePath(catListURL)
	if err := b.writeFile(outputPath, html); err != nil {
		return err
	}

	for _, cat := range b.site.AllCategories {
		totalPosts := len(cat.Posts)
		pageSize := b.cfg.Pagination.PageSize
		if pageSize <= 0 {
			pageSize = 10
		}
		totalPages := int(math.Ceil(float64(totalPosts) / float64(pageSize)))
		if totalPages == 0 {
			totalPages = 1
		}

		for pageNum := 1; pageNum <= totalPages; pageNum++ {
			start := (pageNum - 1) * pageSize
			end := start + pageSize
			if end > totalPosts {
				end = totalPosts
			}

			pagePosts := cat.Posts[start:end]

			pagination := &models.Pagination{
				Page:       pageNum,
				TotalPages: totalPages,
				TotalItems: totalPosts,
				PageSize:   pageSize,
				Items:      pagePosts,
			}

			var currentURL string
			if pageNum == 1 {
				currentURL = cat.URL
			} else {
				currentURL = strings.TrimSuffix(cat.URL, "/")
				if !b.cfg.Build.PrettyURLs {
					currentURL = strings.TrimSuffix(currentURL, ".html")
				}
				currentURL = fmt.Sprintf("%s/page/%d", currentURL, pageNum)
				if b.cfg.Build.PrettyURLs {
					currentURL += "/"
				} else {
					currentURL += ".html"
				}
			}
			pagination.CurrentURL = currentURL

			if pageNum > 1 {
				pagination.HasPrev = true
				if pageNum == 2 {
					pagination.PrevURL = cat.URL
				} else {
					prevURL := strings.TrimSuffix(cat.URL, "/")
					if !b.cfg.Build.PrettyURLs {
						prevURL = strings.TrimSuffix(prevURL, ".html")
					}
					prevURL = fmt.Sprintf("%s/page/%d", prevURL, pageNum-1)
					if b.cfg.Build.PrettyURLs {
						prevURL += "/"
					} else {
						prevURL += ".html"
					}
					pagination.PrevURL = prevURL
				}
			}
			if pageNum < totalPages {
				pagination.HasNext = true
				nextURL := strings.TrimSuffix(cat.URL, "/")
				if !b.cfg.Build.PrettyURLs {
					nextURL = strings.TrimSuffix(nextURL, ".html")
				}
				nextURL = fmt.Sprintf("%s/page/%d", nextURL, pageNum+1)
				if b.cfg.Build.PrettyURLs {
					nextURL += "/"
				} else {
					nextURL += ".html"
				}
				pagination.NextURL = nextURL
			}

			catCtx := &models.TemplateContext{
				Site:         b.site,
				Category:     cat,
				Posts:        pagePosts,
				Pagination:   pagination,
				CurrentURL:   currentURL,
				IsCategory:   true,
				StaticAssets: b.assetMgr.GetAssetMap(),
			}

			var catHTML string
			if b.tmplEngine.HasTemplate("category") {
				catHTML, err = b.tmplEngine.RenderWithBase("base", "category", catCtx)
			} else if b.tmplEngine.HasTemplate("list") {
				catHTML, err = b.tmplEngine.RenderWithBase("base", "list", catCtx)
			} else {
				catHTML, err = b.tmplEngine.Render("list", catCtx)
			}
			if err != nil {
				return fmt.Errorf("render category %s: %w", cat.Name, err)
			}
			catHTML, err = b.maybeMinifyHTML(catHTML)
			if err != nil {
				return err
			}

			outputPath := b.urlToFilePath(currentURL)
			if err := b.writeFile(outputPath, catHTML); err != nil {
				return err
			}
		}
		b.log.Debug("Generated category: %s", cat.Name)
	}

	return nil
}

func (b *Builder) generateTags() error {
	b.log.Info("Generating tag pages...")

	var tagListURL string
	if b.cfg.Build.PrettyURLs {
		tagListURL = "/tags/"
	} else {
		tagListURL = "/tags.html"
	}

	ctx := &models.TemplateContext{
		Site:         b.site,
		CurrentURL:   tagListURL,
		IsTag:        true,
		StaticAssets: b.assetMgr.GetAssetMap(),
	}

	var html string
	var err error

	if b.tmplEngine.HasTemplate("tags") {
		html, err = b.tmplEngine.RenderWithBase("base", "tags", ctx)
	} else if b.tmplEngine.HasTemplate("list") {
		html, err = b.tmplEngine.RenderWithBase("base", "list", ctx)
	} else {
		html, err = b.tmplEngine.Render("list", ctx)
	}
	if err != nil {
		return fmt.Errorf("render tags list: %w", err)
	}
	html, err = b.maybeMinifyHTML(html)
	if err != nil {
		return err
	}
	outputPath := b.urlToFilePath(tagListURL)
	if err := b.writeFile(outputPath, html); err != nil {
		return err
	}

	for _, tag := range b.site.AllTags {
		totalPosts := len(tag.Posts)
		pageSize := b.cfg.Pagination.PageSize
		if pageSize <= 0 {
			pageSize = 10
		}
		totalPages := int(math.Ceil(float64(totalPosts) / float64(pageSize)))
		if totalPages == 0 {
			totalPages = 1
		}

		for pageNum := 1; pageNum <= totalPages; pageNum++ {
			start := (pageNum - 1) * pageSize
			end := start + pageSize
			if end > totalPosts {
				end = totalPosts
			}

			pagePosts := tag.Posts[start:end]

			pagination := &models.Pagination{
				Page:       pageNum,
				TotalPages: totalPages,
				TotalItems: totalPosts,
				PageSize:   pageSize,
				Items:      pagePosts,
			}

			var currentURL string
			if pageNum == 1 {
				currentURL = tag.URL
			} else {
				currentURL = strings.TrimSuffix(tag.URL, "/")
				if !b.cfg.Build.PrettyURLs {
					currentURL = strings.TrimSuffix(currentURL, ".html")
				}
				currentURL = fmt.Sprintf("%s/page/%d", currentURL, pageNum)
				if b.cfg.Build.PrettyURLs {
					currentURL += "/"
				} else {
					currentURL += ".html"
				}
			}
			pagination.CurrentURL = currentURL

			if pageNum > 1 {
				pagination.HasPrev = true
				if pageNum == 2 {
					pagination.PrevURL = tag.URL
				} else {
					prevURL := strings.TrimSuffix(tag.URL, "/")
					if !b.cfg.Build.PrettyURLs {
						prevURL = strings.TrimSuffix(prevURL, ".html")
					}
					prevURL = fmt.Sprintf("%s/page/%d", prevURL, pageNum-1)
					if b.cfg.Build.PrettyURLs {
						prevURL += "/"
					} else {
						prevURL += ".html"
					}
					pagination.PrevURL = prevURL
				}
			}
			if pageNum < totalPages {
				pagination.HasNext = true
				nextURL := strings.TrimSuffix(tag.URL, "/")
				if !b.cfg.Build.PrettyURLs {
					nextURL = strings.TrimSuffix(nextURL, ".html")
				}
				nextURL = fmt.Sprintf("%s/page/%d", nextURL, pageNum+1)
				if b.cfg.Build.PrettyURLs {
					nextURL += "/"
				} else {
					nextURL += ".html"
				}
				pagination.NextURL = nextURL
			}

			tagCtx := &models.TemplateContext{
				Site:         b.site,
				Tag:          tag,
				Posts:        pagePosts,
				Pagination:   pagination,
				CurrentURL:   currentURL,
				IsTag:        true,
				StaticAssets: b.assetMgr.GetAssetMap(),
			}

			var tagHTML string
			if b.tmplEngine.HasTemplate("tag") {
				tagHTML, err = b.tmplEngine.RenderWithBase("base", "tag", tagCtx)
			} else if b.tmplEngine.HasTemplate("list") {
				tagHTML, err = b.tmplEngine.RenderWithBase("base", "list", tagCtx)
			} else {
				tagHTML, err = b.tmplEngine.Render("list", tagCtx)
			}
			if err != nil {
				return fmt.Errorf("render tag %s: %w", tag.Name, err)
			}
			tagHTML, err = b.maybeMinifyHTML(tagHTML)
			if err != nil {
				return err
			}

			outputPath := b.urlToFilePath(currentURL)
			if err := b.writeFile(outputPath, tagHTML); err != nil {
				return err
			}
		}
		b.log.Debug("Generated tag: %s", tag.Name)
	}

	return nil
}

func (b *Builder) generate404() error {
	b.log.Info("Generating 404 page...")

	ctx := &models.TemplateContext{
		Site:         b.site,
		CurrentURL:   "/404.html",
		Is404:        true,
		StaticAssets: b.assetMgr.GetAssetMap(),
	}

	var html string
	var err error

	if b.tmplEngine.HasTemplate("404") {
		html, err = b.tmplEngine.RenderWithBase("base", "404", ctx)
	} else if b.tmplEngine.HasTemplate("page") {
		html, err = b.tmplEngine.RenderWithBase("base", "page", ctx)
	} else {
		html = default404HTML(b.site)
	}
	if err != nil {
		return fmt.Errorf("render 404: %w", err)
	}

	html, err = b.maybeMinifyHTML(html)
	if err != nil {
		return err
	}

	outputPath := filepath.Join(b.cfg.Paths.Public, "404.html")
	if err := b.writeFile(outputPath, html); err != nil {
		return err
	}

	b.log.Debug("Generated 404 page")
	return nil
}

func default404HTML(site *models.Site) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>404 - %s</title>
</head>
<body>
    <h1>404 - 页面未找到</h1>
    <p>抱歉，您访问的页面不存在。</p>
    <p><a href="/">返回首页</a></p>
</body>
</html>`, site.Language, site.Title)
}

func (b *Builder) generateSitemap() error {
	b.log.Info("Generating sitemap.xml...")

	type URL struct {
		Loc        string `xml:"loc"`
		Lastmod    string `xml:"lastmod,omitempty"`
		Changefreq string `xml:"changefreq,omitempty"`
		Priority   string `xml:"priority,omitempty"`
	}

	type Sitemap struct {
		XMLName xml.Name `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
		URLs    []URL    `xml:"url"`
	}

	sitemap := &Sitemap{
		URLs: []URL{},
	}

	baseURL := strings.TrimSuffix(b.cfg.Site.URL, "/")

	addURL := func(loc string, lastmod time.Time, priority string) {
		fullURL := baseURL + loc
		sitemap.URLs = append(sitemap.URLs, URL{
			Loc:        fullURL,
			Lastmod:    lastmod.Format("2006-01-02"),
			Changefreq: "weekly",
			Priority:   priority,
		})
	}

	addURL("/", b.site.BuildTime, "1.0")

	for _, post := range b.posts {
		addURL(post.URL, post.Date, "0.8")
	}

	for _, page := range b.pages {
		addURL(page.URL, b.site.BuildTime, "0.7")
	}

	if b.cfg.Build.PrettyURLs {
		addURL("/archive/", b.site.BuildTime, "0.6")
		addURL("/categories/", b.site.BuildTime, "0.5")
		addURL("/tags/", b.site.BuildTime, "0.5")
	} else {
		addURL("/archive.html", b.site.BuildTime, "0.6")
		addURL("/categories.html", b.site.BuildTime, "0.5")
		addURL("/tags.html", b.site.BuildTime, "0.5")
	}

	for _, cat := range b.site.AllCategories {
		addURL(cat.URL, b.site.BuildTime, "0.5")
	}

	for _, tag := range b.site.AllTags {
		addURL(tag.URL, b.site.BuildTime, "0.4")
	}

	data, err := xml.MarshalIndent(sitemap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sitemap: %w", err)
	}

	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + string(data) + "\n"
	outputPath := filepath.Join(b.cfg.Paths.Public, "sitemap.xml")
	if err := b.writeFile(outputPath, xmlContent); err != nil {
		return err
	}

	b.log.Debug("Generated sitemap.xml")
	return nil
}

func (b *Builder) generateRSS() error {
	b.log.Info("Generating RSS feed...")

	type Item struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		PubDate     string `xml:"pubDate"`
		Description string `xml:"description"`
		GUID        string `xml:"guid"`
		Category    string `xml:"category,omitempty"`
	}

	type Channel struct {
		Title          string `xml:"title"`
		Link           string `xml:"link"`
		Description    string `xml:"description"`
		Language       string `xml:"language"`
		LastBuildDate  string `xml:"lastBuildDate"`
		Generator      string `xml:"generator"`
		Items          []Item `xml:"item"`
	}

	type RSS struct {
		XMLName xml.Name `xml:"rss"`
		Version string   `xml:"version,attr"`
		Channel Channel  `xml:"channel"`
	}

	baseURL := strings.TrimSuffix(b.cfg.Site.URL, "/")
	rssURL := baseURL + "/index.xml"

	rss := &RSS{
		Version: "2.0",
		Channel: Channel{
			Title:         b.site.Title,
			Link:          baseURL,
			Description:   b.site.Description,
			Language:      b.site.Language,
			LastBuildDate: time.Now().Format(time.RFC1123Z),
			Generator:     "staticgen",
			Items:         []Item{},
		},
	}

	maxItems := 20
	if len(b.posts) < maxItems {
		maxItems = len(b.posts)
	}

	for i := 0; i < maxItems; i++ {
		post := b.posts[i]
		item := Item{
			Title:       post.Title,
			Link:        baseURL + post.URL,
			PubDate:     post.Date.Format(time.RFC1123Z),
			Description: html.EscapeString(post.Summary),
			GUID:        baseURL + post.URL,
		}
		if len(post.Categories) > 0 {
			item.Category = post.Categories[0]
		}
		rss.Channel.Items = append(rss.Channel.Items, item)
	}

	data, err := xml.MarshalIndent(rss, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal rss: %w", err)
	}

	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + string(data) + "\n"
	outputPath := filepath.Join(b.cfg.Paths.Public, "index.xml")
	if err := b.writeFile(outputPath, xmlContent); err != nil {
		return err
	}

	b.log.Debug("Generated RSS feed at %s", rssURL)
	return nil
}

func (b *Builder) generateRobots() error {
	b.log.Info("Generating robots.txt...")

	baseURL := strings.TrimSuffix(b.cfg.Site.URL, "/")

	content := fmt.Sprintf(`User-agent: *
Allow: /

Sitemap: %s/sitemap.xml
`, baseURL)

	outputPath := filepath.Join(b.cfg.Paths.Public, "robots.txt")
	if err := b.writeFile(outputPath, content); err != nil {
		return err
	}

	b.log.Debug("Generated robots.txt")
	return nil
}
