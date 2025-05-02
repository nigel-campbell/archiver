// Package document provides data structures for representing web documents and pages
package document

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

type Field struct {
	name     string
	value    string
	analyzed bool
}

// Document attempts to provide "structure" to the crawled pages.
// This struct is mostly unused and intended for a future "indexing" usecase.
type Document struct {
	id     string
	fields []Field
}

func NewEmptyDocument(id string) Document {
	return Document{
		id:     id,
		fields: make([]Field, 0),
	}
}

func NewTextField(name string, value string, analyzed bool) Field {
	return Field{
		name:     name,
		value:    value,
		analyzed: analyzed,
	}
}

func (d *Document) AddField(f Field) {
	d.fields = append(d.fields, f)
}

func (d *Document) GetField(name string) string {
	for _, f := range d.fields {
		if f.name == name {
			return f.value
		}
	}
	return ""
}

func (d *Document) GetFields(name string) []string {
	var values []string
	for _, f := range d.fields {
		if f.name == name {
			values = append(values, f.value)
		}
	}
	return values
}

func (d *Document) ID() string {
	return d.id
}

func (d *Document) Fields() []Field {
	return d.fields
}

func PageToDocument(p *RawPage) (Document, error) {
	root, err := html.Parse(bytes.NewReader(p.Content))
	if err != nil {
		return Document{}, fmt.Errorf("html parse error: %w", err)
	}

	extracted := extractFromHTML(root)

	doc := NewEmptyDocument(p.URL) // assuming URL is your doc ID

	doc.AddField(NewTextField("url", p.URL, false))
	doc.AddField(NewTextField("title", extracted.Title, true))
	doc.AddField(NewTextField("body", extracted.Text, true))

	for name, content := range extracted.Meta {
		doc.AddField(NewTextField("meta_"+name, content, true))
	}

	for _, header := range extracted.Headers {
		doc.AddField(NewTextField("header", header, true))
	}

	return doc, nil
}

type Extracted struct {
	Title   string
	Text    string
	Meta    map[string]string // e.g., description, keywords
	Headers []string          // h1, h2, h3...
}

func extractFromHTML(root *html.Node) Extracted {
	var result Extracted
	result.Meta = make(map[string]string)
	var buf strings.Builder
	var inTitle bool

	var walker func(*html.Node)
	walker = func(n *html.Node) {
		switch n.Type {
		case html.ElementNode:
			switch n.Data {
			case "title":
				inTitle = true
			case "meta":
				var name, content string
				for _, attr := range n.Attr {
					switch strings.ToLower(attr.Key) {
					case "name":
						name = strings.ToLower(attr.Val)
					case "content":
						content = attr.Val
					}
				}
				if name != "" && content != "" {
					result.Meta[name] = content
				}
			case "h1", "h2", "h3":
				if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					text := strings.TrimSpace(n.FirstChild.Data)
					if text != "" {
						result.Headers = append(result.Headers, text)
					}
				}
			case "script", "style":
				return // skip
			}
		case html.TextNode:
			data := strings.TrimSpace(n.Data)
			if data != "" {
				if inTitle {
					result.Title += data + " "
				} else {
					buf.WriteString(data + " ")
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walker(c)
		}

		if n.Type == html.ElementNode && n.Data == "title" {
			inTitle = false
		}
	}

	walker(root)

	result.Title = strings.TrimSpace(result.Title)
	result.Text = strings.TrimSpace(buf.String())
	return result
}

// RawPage represents a crawled web page
type RawPage struct {
	URL     string // The URL of the page
	Content []byte // The raw content of the page
}

func (p *RawPage) String() string {
	return string(p.Content)
}

// Links returns the set of links parsed from the raw contents of the page
func (p *RawPage) Links() ([]string, error) {
	doc, err := html.Parse(bytes.NewReader(p.Content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse url %s: %w", p.URL, err)
	}

	var links []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					// Skip special URLs
					if strings.HasPrefix(attr.Val, "javascript:") ||
						strings.HasPrefix(attr.Val, "mailto:") ||
						strings.HasPrefix(attr.Val, "tel:") ||
						strings.HasPrefix(attr.Val, "#") {
						continue
					}
					links = append(links, attr.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	fmt.Printf("Found %d links on page %s\n", len(links), p.URL)
	return links, nil
}

func NewRawPage(url string, content []byte) *RawPage {
	return &RawPage{
		URL:     url,
		Content: content,
	}
}

func (f Field) Value() string {
	return f.value
}

func (f Field) Analyzed() bool {
	return f.analyzed
}
