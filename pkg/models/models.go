package models

import (
	"time"
)

type Post struct {
	Title      string
	Slug       string
	Date       time.Time
	Tags       []string
	Categories []string
	Draft      bool
	Summary    string
	Cover      string
	Author     string
	Content    string
	HTML       string
	Path       string
	URL        string
	FileName   string
	WordCount  int
	ReadingTime int
	Previous   *Post
	Next       *Post
}

type Page struct {
	Title   string
	Content string
	HTML    string
	Path    string
	URL     string
	Data    map[string]interface{}
}

type Category struct {
	Name  string
	URL   string
	Count int
	Posts []*Post
}

type Tag struct {
	Name  string
	URL   string
	Count int
	Posts []*Post
}

type Pagination struct {
	Page        int
	TotalPages  int
	TotalItems  int
	PageSize    int
	HasPrev     bool
	HasNext     bool
	PrevURL     string
	NextURL     string
	FirstURL    string
	LastURL     string
	CurrentURL  string
	Items       interface{}
}

type Site struct {
	Title       string
	Description string
	Author      string
	Language    string
	BaseURL     string
	URL         string
	Copyright   string
	Keywords    []string
	Posts       []*Post
	Pages       []*Page
	Categories  map[string]*Category
	Tags        map[string]*Tag
	RecentPosts []*Post
	AllTags     []*Tag
	AllCategories []*Category
	BuildTime   time.Time
}

type TemplateContext struct {
	Site         *Site
	Page         interface{}
	Post         *Post
	Posts        []*Post
	Pagination   *Pagination
	Category     *Category
	Tag          *Tag
	Data         map[string]interface{}
	CurrentURL   string
	IsHome       bool
	IsArchive    bool
	IsCategory   bool
	IsTag        bool
	IsPage       bool
	IsPost       bool
	Is404        bool
	StaticAssets map[string]string
}
