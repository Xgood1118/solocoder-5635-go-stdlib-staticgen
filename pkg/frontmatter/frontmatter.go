package frontmatter

import (
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type FrontMatter struct {
	Title      string    `yaml:"title"`
	Date       string    `yaml:"date"`
	Tags       []string  `yaml:"tags"`
	Categories []string  `yaml:"categories"`
	Draft      bool      `yaml:"draft"`
	Summary    string    `yaml:"summary"`
	Cover      string    `yaml:"cover"`
	Author     string    `yaml:"author"`
}

type ParseResult struct {
	FrontMatter FrontMatter
	Content     string
}

func Parse(data []byte) (*ParseResult, error) {
	content := string(data)
	result := &ParseResult{}

	if strings.HasPrefix(content, "---") {
		end := strings.Index(content[3:], "---")
		if end != -1 {
			fmContent := content[3 : end+3]
			remaining := content[end+6:]
			
			var fm FrontMatter
			if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
				return nil, err
			}
			result.FrontMatter = fm
			result.Content = strings.TrimSpace(remaining)
			return result, nil
		}
	}

	result.Content = strings.TrimSpace(content)
	return result, nil
}

func ParseDate(dateStr string) (time.Time, error) {
	layouts := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, dateStr, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Now(), nil
}
