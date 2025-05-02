package crawler

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nigel-campbell/archiver/internal/document"
)

const userAgent = "ArchiverBot/0.1"

// Crawler handles the web crawling logic
type Crawler struct {
	visited      sync.Map
	queue        []queueItem
	mu           sync.Mutex
	pagesDir     string
	urlMappings  map[string]string // Maps hashed filenames to original URLs
	mapMutex     sync.Mutex        // Protects the urlMappings map
	userAgent    string            // User agent for HTTP requests
	pagesSaved   int               // Counter for saved pages
	pagesCrawled int               // Counter for successfully crawled pages
	saveToDisk   bool              // Flag to control whether pages are saved to disk
	workerCount  int               // Number of concurrent workers
	maxDepth     int               // Maximum depth to crawl
	crawlTime    time.Time         // When the crawl started
}

type queueItem struct {
	url   string
	depth int
}

func NewCrawler() *Crawler {
	return &Crawler{
		queue:        make([]queueItem, 0),
		pagesDir:     "pages",
		urlMappings:  make(map[string]string),
		userAgent:    userAgent,
		pagesSaved:   0,
		pagesCrawled: 0,
		saveToDisk:   false,
		workerCount:  5,
		maxDepth:     -1, // -1 means no depth limit
	}
}

func (c *Crawler) SetPagesDirectory(dir string) {
	c.pagesDir = dir
}

func (c *Crawler) SetUserAgent(userAgent string) {
	c.userAgent = userAgent
}

func (c *Crawler) SetSavePagesToDisk(save bool) {
	c.saveToDisk = save
}

func (c *Crawler) SetWorkerCount(count int) {
	if count > 0 {
		c.workerCount = count
	}
}

func (c *Crawler) SetMaxDepth(depth int) {
	c.maxDepth = depth
}

func extractHostname(domain string) (string, error) {
	if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
		domain = "https://" + domain
	}
	u, err := url.Parse(domain)
	if err != nil {
		return "", err
	}
	return u.Hostname(), nil
}

// Crawl attempts to crawl the set of pages for a given domain starting from the root. It excludes the contents of other
// domains, opting to add those domains to the "frontier" of what to crawl next.
func (c *Crawler) Crawl(startUrl string) error {
	if c.saveToDisk {
		c.crawlTime = time.Now()
		if err := c.ensurePagesDirectory(); err != nil {
			return fmt.Errorf("failed to create pages directory: %w", err)
		}
	}

	domainHostname, err := extractHostname(startUrl)
	if err != nil {
		return fmt.Errorf("failed to parse domain: %w", err)
	}

	fmt.Printf("Starting crawl for domain hostname: %s with %d workers\n", domainHostname, c.workerCount)

	c.addToQueue(startUrl, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	urlChan := make(chan queueItem, 100)

	var wg sync.WaitGroup

	for i := 0; i < c.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-urlChan:
					if !ok {
						return
					}
					if err := c.crawl(domainHostname, item.url, item.depth); err != nil {
						log.Printf("Worker %d failed to crawl %s: %v\n", workerID, item.url, err)
					}
				}
			}
		}(i)
	}

	go func() {
		defer close(urlChan)
		for {
			url, depth := c.popFromQueue()
			if url == "" {
				time.Sleep(500 * time.Millisecond)
				if len(c.queue) == 0 {
					return
				}
				continue
			}

			select {
			case <-ctx.Done():
				return
			case urlChan <- queueItem{url: url, depth: depth}:
			}
		}
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.mu.Lock()
				queueSize := len(c.queue)
				crawled := c.pagesCrawled
				saved := c.pagesSaved
				c.mu.Unlock()
				fmt.Printf("Status: Queue size: %d, Pages crawled: %d, Pages saved: %d\n", queueSize, crawled, saved)
			}
		}
	}()

	wg.Wait()

	fmt.Printf("Crawl complete. Successfully crawled %d pages.", c.pagesCrawled)
	if c.saveToDisk {
		fmt.Printf(" Saved %d pages to disk.\n", c.pagesSaved)

		if err := c.saveURLMappings(); err != nil {
			fmt.Printf("Error saving URL mappings: %v\n", err)
		}
	} else {
		fmt.Println()
	}

	return nil
}

func (c *Crawler) crawl(domainHostname string, url string, currentDepth int) error {
	if c.maxDepth >= 0 && currentDepth > c.maxDepth {
		return nil
	}

	if _, loaded := c.visited.LoadOrStore(url, true); loaded {
		return nil
	}

	fmt.Printf("Crawling: %s (depth: %d)\n", url, currentDepth)
	page, err := c.fetch(url)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", url, err)
	}

	c.mu.Lock()
	c.pagesCrawled++
	c.mu.Unlock()

	if c.saveToDisk {
		if err := c.savePage(page); err != nil {
			return fmt.Errorf("failed to save page %s: %w", url, err)
		} else {
			c.mu.Lock()
			c.pagesSaved++
			c.mu.Unlock()
		}
	}

	links, err := page.Links()
	if err != nil {
		return fmt.Errorf("Error extracting links from %s: %v\n", url, err)
	}

	for _, link := range links {
		if link == "" || strings.HasPrefix(link, "javascript:") ||
			strings.HasPrefix(link, "mailto:") || strings.HasPrefix(link, "tel:") ||
			strings.HasPrefix(link, "#") {
			continue
		}

		absoluteURL, err := normalizeURL(url, link)
		if err != nil {
			fmt.Printf("Error normalizing URL %s: %v\n", link, err)
			continue
		}

		linkHostname, err := extractHostname(absoluteURL)
		if err != nil {
			fmt.Printf("Error extracting hostname from %s: %v\n", absoluteURL, err)
			continue
		}

		if linkHostname == domainHostname {
			if _, visited := c.visited.Load(absoluteURL); !visited {
				c.addToQueue(absoluteURL, currentDepth+1)
			}
		}
	}

	return nil
}

func (c *Crawler) ensurePagesDirectory() error {
	// Create the base directory if it doesn't exist
	if err := os.MkdirAll(c.pagesDir, 0755); err != nil {
		return err
	}

	// Create a timestamped subdirectory for this crawl
	timestamp := c.crawlTime.Format("2006-01-02_15-04-05")
	crawlDir := filepath.Join(c.pagesDir, timestamp)
	return os.MkdirAll(crawlDir, 0755)
}

func hashURL(urlStr string) string {
	hash := md5.Sum([]byte(urlStr))
	return hex.EncodeToString(hash[:8]) // Use only first 8 bytes (16 hex chars) to keep it shorter
}

// savePage writes the page content to disk with an obfuscated filename
// TODO(nigel): Is there any actual value in obfuscating the filename?
func (c *Crawler) savePage(page *document.RawPage) error {
	// Generate a hashed filename
	hashedName := hashURL(page.URL)
	filename := hashedName + ".html"

	// Create the full path including the timestamped directory
	timestamp := c.crawlTime.Format("2006-01-02_15-04-05")
	crawlDir := filepath.Join(c.pagesDir, timestamp)
	filePath := filepath.Join(crawlDir, filename)

	// Save the mapping of hash to original URL
	c.mapMutex.Lock()
	c.urlMappings[hashedName] = page.URL
	c.mapMutex.Unlock()

	// Write the file
	return os.WriteFile(filePath, page.Content, 0644)
}

func (c *Crawler) saveURLMappings() error {
	if !c.saveToDisk {
		return nil
	}

	c.mapMutex.Lock()
	defer c.mapMutex.Unlock()

	// Save mappings in the timestamped directory
	timestamp := c.crawlTime.Format("2006-01-02_15-04-05")
	crawlDir := filepath.Join(c.pagesDir, timestamp)
	mappingsFile := filepath.Join(crawlDir, "url_mappings.json")
	mappingsData, err := json.MarshalIndent(c.urlMappings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(mappingsFile, mappingsData, 0644)
}

func (c *Crawler) GetOriginalURL(hashedName string) (string, bool) {
	c.mapMutex.Lock()
	defer c.mapMutex.Unlock()

	// Remove .html extension if present
	hashedName = strings.TrimSuffix(hashedName, ".html")
	url, exists := c.urlMappings[hashedName]
	return url, exists
}

func normalizeURL(baseURLStr, linkURLStr string) (string, error) {
	// Skip special URLs
	if strings.HasPrefix(linkURLStr, "javascript:") ||
		strings.HasPrefix(linkURLStr, "mailto:") ||
		strings.HasPrefix(linkURLStr, "tel:") ||
		strings.HasPrefix(linkURLStr, "#") {
		return "", fmt.Errorf("special URL scheme not supported: %s", linkURLStr)
	}

	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return "", err
	}

	linkURL, err := url.Parse(linkURLStr)
	if err != nil {
		return "", err
	}

	return baseURL.ResolveReference(linkURL).String(), nil
}

func (c *Crawler) fetch(url string) (*document.RawPage, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)

	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	rsp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the page contents for the given URL: %w", err)
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK response status: %s", rsp.Status)
	}

	contentType := rsp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return nil, fmt.Errorf("skipping non-HTML content type: %s", contentType)
	}

	b, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}
	return document.NewRawPage(url, b), nil
}

func (c *Crawler) Start(ctx context.Context, domains ...string) error {
	if len(domains) == 0 {
		return fmt.Errorf("no domains provided for crawling")
	}

	for _, domain := range domains {
		if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
			domain = "https://" + domain
		}

		fmt.Printf("Starting crawl for URL: %s\n", domain)

		if err := c.Crawl(domain); err != nil {
			fmt.Printf("Error crawling %s: %v\n", domain, err)
		}
	}

	return nil
}

func (c *Crawler) addToQueue(url string, depth int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queue = append(c.queue, queueItem{url: url, depth: depth})
}

func (c *Crawler) popFromQueue() (string, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.queue) == 0 {
		return "", 0
	}
	item := c.queue[0]
	c.queue = c.queue[1:]
	return item.url, item.depth
}
