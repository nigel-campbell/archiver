package document

import (
	"strings"
	"testing"
)

func TestNewEmptyDocument(t *testing.T) {
	doc := NewEmptyDocument("test-id")
	if doc.ID() != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", doc.ID())
	}
	if len(doc.Fields()) != 0 {
		t.Errorf("Expected empty fields, got %d fields", len(doc.Fields()))
	}
}

func TestDocumentFieldManagement(t *testing.T) {
	doc := NewEmptyDocument("test-id")

	// Test adding and retrieving a single field
	field := NewTextField("test", "value", true)
	doc.AddField(field)

	if doc.GetField("test") != "value" {
		t.Errorf("Expected field value 'value', got '%s'", doc.GetField("test"))
	}

	// Test retrieving non-existent field
	if doc.GetField("nonexistent") != "" {
		t.Error("Expected empty string for nonexistent field")
	}

	// Test multiple fields with same name
	doc.AddField(NewTextField("test", "value2", true))
	values := doc.GetFields("test")
	if len(values) != 2 {
		t.Errorf("Expected 2 values, got %d", len(values))
	}
	if values[0] != "value" || values[1] != "value2" {
		t.Errorf("Expected values ['value', 'value2'], got %v", values)
	}
}

func TestPageToDocument(t *testing.T) {
	// Test with a simple HTML page
	htmlContent := `
		<html>
			<head>
				<title>Test Page</title>
				<meta name="description" content="Test description">
			</head>
			<body>
				<h1>Main Heading</h1>
				<p>Some text content</p>
			</body>
		</html>
	`

	page := NewRawPage("http://example.com", []byte(htmlContent))
	doc, err := PageToDocument(page)
	if err != nil {
		t.Fatalf("Failed to convert page to document: %v", err)
	}

	// Test title extraction
	if doc.GetField("title") != "Test Page" {
		t.Errorf("Expected title 'Test Page', got '%s'", doc.GetField("title"))
	}

	// Test meta tag extraction
	if doc.GetField("meta_description") != "Test description" {
		t.Errorf("Expected meta description 'Test description', got '%s'", doc.GetField("meta_description"))
	}

	// Test header extraction
	headers := doc.GetFields("header")
	if len(headers) != 1 || headers[0] != "Main Heading" {
		t.Errorf("Expected header 'Main Heading', got %v", headers)
	}

	// Test body text extraction
	body := doc.GetField("body")
	if !strings.Contains(body, "Some text content") {
		t.Errorf("Expected body to contain 'Some text content', got '%s'", body)
	}
}

func TestRawPageLinks(t *testing.T) {
	// Test with HTML containing links
	htmlContent := `
		<html>
			<body>
				<a href="https://example.com">Example</a>
				<a href="/relative">Relative</a>
				<a href="javascript:void(0)">JavaScript</a>
				<a href="mailto:test@example.com">Email</a>
				<a href="tel:+1234567890">Phone</a>
				<a href="#section">Anchor</a>
			</body>
		</html>
	`

	page := NewRawPage("http://example.com", []byte(htmlContent))
	links, err := page.Links()
	if err != nil {
		t.Fatalf("Failed to extract links: %v", err)
	}

	expectedLinks := []string{"https://example.com", "/relative"}
	if len(links) != len(expectedLinks) {
		t.Errorf("Expected %d links, got %d", len(expectedLinks), len(links))
	}

	for i, link := range expectedLinks {
		if links[i] != link {
			t.Errorf("Expected link '%s', got '%s'", link, links[i])
		}
	}
}
