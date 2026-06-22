package markdown

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	ghtml "github.com/yuin/goldmark/renderer/html"
)

var (
	mdLinkRegex  = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\.md(#[^)]+)?\)`)
	mdImgRegex   = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	codeBlockRegex = regexp.MustCompile(`(?s)<pre><code class="language-([^"]+)">([\s\S]*?)</code></pre>`)
	simpleCodeBlockRegex = regexp.MustCompile(`(?s)<pre><code>([\s\S]*?)</code></pre>`)
)

type Converter struct {
	md         goldmark.Markdown
	contentDir string
	prettyURLs bool
}

func NewConverter(contentDir string, prettyURLs bool) *Converter {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
			extension.Linkify,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			ghtml.WithHardWraps(),
			ghtml.WithXHTML(),
		),
	)

	return &Converter{
		md:         md,
		contentDir: contentDir,
		prettyURLs: prettyURLs,
	}
}

func (c *Converter) Convert(source []byte, currentPath string) (string, error) {
	content := string(source)
	content = c.convertInternalLinks(content, currentPath)
	content = c.convertImagePaths(content, currentPath)

	var buf bytes.Buffer
	if err := c.md.Convert([]byte(content), &buf); err != nil {
		return "", fmt.Errorf("markdown convert error: %w", err)
	}

	htmlOutput := buf.String()
	htmlOutput = c.highlightCodeBlocks(htmlOutput)

	return htmlOutput, nil
}

func (c *Converter) highlightCodeBlocks(html string) string {
	html = codeBlockRegex.ReplaceAllStringFunc(html, func(match string) string {
		matches := codeBlockRegex.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		lang := matches[1]
		code := decodeHTMLEntities(matches[2])

		highlighted, err := HighlightCode(code, lang)
		if err != nil {
			return match
		}
		return highlighted
	})

	html = simpleCodeBlockRegex.ReplaceAllStringFunc(html, func(match string) string {
		matches := simpleCodeBlockRegex.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		code := decodeHTMLEntities(matches[1])

		highlighted, err := HighlightCode(code, "")
		if err != nil {
			return match
		}
		return highlighted
	})

	return html
}

func decodeHTMLEntities(s string) string {
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return s
}

func (c *Converter) convertInternalLinks(content string, currentPath string) string {
	return mdLinkRegex.ReplaceAllStringFunc(content, func(match string) string {
		submatches := mdLinkRegex.FindStringSubmatch(match)
		if len(submatches) < 4 {
			return match
		}

		linkText := submatches[1]
		linkPath := submatches[2]
		anchor := submatches[3]

		targetURL := c.resolveMDToHTML(linkPath, currentPath)
		return fmt.Sprintf("[%s](%s%s)", linkText, targetURL, anchor)
	})
}

func (c *Converter) convertImagePaths(content string, currentPath string) string {
	return mdImgRegex.ReplaceAllStringFunc(content, func(match string) string {
		submatches := mdImgRegex.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		altText := submatches[1]
		imgPath := submatches[2]

		if strings.HasPrefix(imgPath, "http://") || strings.HasPrefix(imgPath, "https://") || strings.HasPrefix(imgPath, "//") || strings.HasPrefix(imgPath, "/") {
			return match
		}

		resolvedPath := c.resolveImagePath(imgPath, currentPath)
		return fmt.Sprintf("![%s](%s)", altText, resolvedPath)
	})
}

func (c *Converter) resolveMDToHTML(mdPath string, currentPath string) string {
	if strings.HasPrefix(mdPath, "/") {
		cleanPath := strings.TrimPrefix(mdPath, "/")
		return c.mdToHTMLOutputPath(cleanPath)
	}

	currentDir := filepath.Dir(currentPath)
	fullPath := filepath.Clean(filepath.Join(currentDir, mdPath))
	relPath, err := filepath.Rel(c.contentDir, fullPath)
	if err != nil {
		return c.mdToHTMLOutputPath(mdPath)
	}
	return c.mdToHTMLOutputPath(relPath)
}

func (c *Converter) mdToHTMLOutputPath(mdPath string) string {
	mdPath = filepath.ToSlash(mdPath)
	mdPath = strings.TrimSuffix(mdPath, ".md")

	parts := strings.Split(mdPath, "/")
	last := parts[len(parts)-1]
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}-`, last); matched {
		parts[len(parts)-1] = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-`).ReplaceAllString(last, "")
		mdPath = strings.Join(parts, "/")
	}

	if c.prettyURLs {
		if mdPath == "index" || mdPath == "" {
			return "/"
		}
		return "/" + mdPath + "/"
	}
	return "/" + mdPath + ".html"
}

func (c *Converter) resolveImagePath(imgPath string, currentPath string) string {
	currentDir := filepath.Dir(currentPath)
	fullPath := filepath.Clean(filepath.Join(currentDir, imgPath))
	relPath, err := filepath.Rel(c.contentDir, fullPath)
	if err != nil {
		return "/" + filepath.ToSlash(imgPath)
	}
	return "/" + filepath.ToSlash(relPath)
}

func CountWords(content string) int {
	content = strings.TrimSpace(content)
	if content == "" {
		return 0
	}

	chineseCount := 0
	for _, r := range content {
		if r >= 0x4e00 && r <= 0x9fff {
			chineseCount++
		}
	}

	nonChinese := regexp.MustCompile(`[\p{Han}]`).ReplaceAllString(content, " ")
	words := strings.Fields(nonChinese)

	return chineseCount + len(words)
}

func EstimateReadingTime(wordCount int) int {
	cpm := 300
	minutes := wordCount / cpm
	if wordCount%cpm > 0 {
		minutes++
	}
	if minutes < 1 {
		minutes = 1
	}
	return minutes
}

func HighlightCode(code, lang string) (string, error) {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("github")
	if style == nil {
		style = styles.Fallback
	}

	formatter := html.New(html.WithClasses(true))

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
