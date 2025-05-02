package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/nigel-campbell/archiver/internal/crawler"
)

var (
	persist = flag.Bool("persist", false, "whether or not to persist to disk")
	workers = flag.Int("workers", 3, "number of workers")
	dir     = flag.String("dir", "archive", "directory to save pages to")
	domains = flag.String("domain", "https://example.com", "common separated list of domains to crawl")
	depth   = flag.Int("depth", -1, "maximum depth to crawl (-1 for unlimited)")
)

func main() {
	flag.Parse()
	c := crawler.NewCrawler()

	c.SetSavePagesToDisk(*persist)
	c.SetWorkerCount(*workers)
	c.SetPagesDirectory(*dir)
	c.SetMaxDepth(*depth)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	d := strings.Split(*domains, ",")
	for _, domain := range d {
		if !isValid(domain) {
			log.Fatalf("Invalid domain: %s", domain)
		}
	}

	if err := c.Start(ctx, d...); err != nil {
		log.Fatalf("Error during crawl: %v", err)
	}
}

func isValid(domain string) bool {
	if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
		domain = "https://" + domain
	}

	u, err := url.Parse(domain)
	if err != nil {
		return false
	}

	if u.Hostname() == "" {
		return false
	}

	if !strings.Contains(u.Hostname(), ".") {
		return false
	}
	return true
}
