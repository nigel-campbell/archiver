package crawler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestURLNormalization(t *testing.T) {
	tests := []struct {
		baseURL   string
		linkURL   string
		expected  string
		shouldErr bool
	}{
		{
			baseURL:   "https://example.com",
			linkURL:   "/page",
			expected:  "https://example.com/page",
			shouldErr: false,
		},
		{
			baseURL:   "https://example.com/",
			linkURL:   "page",
			expected:  "https://example.com/page",
			shouldErr: false,
		},
		{
			baseURL:   "https://example.com/dir/",
			linkURL:   "../page",
			expected:  "https://example.com/page",
			shouldErr: false,
		},
		{
			baseURL:   "https://example.com",
			linkURL:   "https://other.com/page",
			expected:  "https://other.com/page",
			shouldErr: false,
		},
		{
			baseURL:   "https://example.com",
			linkURL:   "javascript:void(0)",
			expected:  "",
			shouldErr: true,
		},
		{
			baseURL:   "https://example.com",
			linkURL:   "mailto:test@example.com",
			expected:  "",
			shouldErr: true,
		},
		{
			baseURL:   "https://example.com",
			linkURL:   "tel:+1234567890",
			expected:  "",
			shouldErr: true,
		},
		{
			baseURL:   "https://example.com",
			linkURL:   "#section",
			expected:  "",
			shouldErr: true,
		},
	}

	for _, test := range tests {
		result, err := normalizeURL(test.baseURL, test.linkURL)
		if test.shouldErr {
			if err == nil {
				t.Errorf("Expected error for baseURL=%s, linkURL=%s", test.baseURL, test.linkURL)
			}
			continue
		}
		if err != nil {
			t.Errorf("Unexpected error for baseURL=%s, linkURL=%s: %v", test.baseURL, test.linkURL, err)
			continue
		}
		if result != test.expected {
			t.Errorf("Expected %s, got %s for baseURL=%s, linkURL=%s", test.expected, result, test.baseURL, test.linkURL)
		}
	}
}

func TestCrawlerDepthLimit(t *testing.T) {
	// Create a test server that returns a simple HTML page with links
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<body>
					<a href="/page1">Page 1</a>
					<a href="/page2">Page 2</a>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "crawler-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	c := NewCrawler()
	c.SetPagesDirectory(tempDir)
	c.SetSavePagesToDisk(true)
	c.SetMaxDepth(1) // Limit depth to 1
	c.SetWorkerCount(1)

	// Start crawling
	err = c.Crawl(server.URL)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	// Check that we only crawled the root page and immediate links
	// The exact count might vary based on implementation, but should be limited
	if c.pagesCrawled > 3 { // root + 2 links
		t.Errorf("Expected at most 3 pages crawled with depth 1, got %d", c.pagesCrawled)
	}
}

func TestCrawlerFileSaving(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<head>
					<title>Test Page</title>
				</head>
				<body>
					<h1>Hello World</h1>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "crawler-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	c := NewCrawler()
	c.SetPagesDirectory(tempDir)
	c.SetSavePagesToDisk(true)
	c.SetMaxDepth(0) // Only crawl the root page
	c.SetWorkerCount(1)

	// Start crawling
	err = c.Crawl(server.URL)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp directory: %v", err)
	}

	if len(files) == 0 {
		t.Error("Expected at least one saved file, got none")
	}

	timestamp := c.crawlTime.Format("2006-01-02_15-04-05")
	crawlDir := filepath.Join(tempDir, timestamp)
	mappingsFile := filepath.Join(crawlDir, "url_mappings.json")
	if _, err := os.Stat(mappingsFile); os.IsNotExist(err) {
		t.Error("Expected url_mappings.json to exist in timestamped directory")
	}
}

func TestCrawlerConcurrency(t *testing.T) {
	// Create a test server that simulates slow responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Simulate network delay
		w.Write([]byte(`
			<html>
				<body>
					<a href="/page1">Page 1</a>
					<a href="/page2">Page 2</a>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "crawler-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// First test with a single worker
	c1 := NewCrawler()
	c1.SetPagesDirectory(tempDir)
	c1.SetSavePagesToDisk(true)
	c1.SetMaxDepth(1)
	c1.SetWorkerCount(1)

	startTime1 := time.Now()
	err = c1.Crawl(server.URL)
	if err != nil {
		t.Fatalf("Crawl failed with single worker: %v", err)
	}
	singleWorkerDuration := time.Since(startTime1)

	c2 := NewCrawler()
	c2.SetPagesDirectory(tempDir)
	c2.SetSavePagesToDisk(true)
	c2.SetMaxDepth(1)
	c2.SetWorkerCount(3)

	startTime2 := time.Now()
	err = c2.Crawl(server.URL)
	if err != nil {
		t.Fatalf("Crawl failed with multiple workers: %v", err)
	}
	multiWorkerDuration := time.Since(startTime2)

	if multiWorkerDuration >= singleWorkerDuration {
		t.Errorf("Expected concurrent crawl to be faster. Single worker: %v, Multi worker: %v", singleWorkerDuration, multiWorkerDuration)
	}
}
