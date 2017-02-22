package main

import (
	"fmt"
	"testing"
)

// testFetcher is a simple map of URLs to links in that URL
type testFetcher map[string][]string

// Get returns a test response for the given URL; implements Fetcher.Get
func (t testFetcher) Get(url string) ([]string, error) {
	if links, ok := t[url]; ok {
		return links, nil
	}
	return nil, fmt.Errorf("could not find URL %s", url)
}

var tester = testFetcher{
	"http://golang.org": []string{
		"http://golang.org/test.html",
		"http://golang.org/test2.html",
		"http://golang.org/images/gopher.jpg",
		"http://goweeklynews.com", // Won't get crawled
	},
	"http://golang.org/test.html": []string{
		"http://golang.org/testing.html",
		"http://golang.org/images/gopher.jpg",
		"http://linkedin.com/golang",
	},
	"http://golang.org/test2.html": []string{
		"http://golang.org", //  Won't get crawled

	},
	"http://golang.org/testing.html": []string{
		"http://golang.org", // Won't get crawled
	},
}

func TestCrawler(t *testing.T) {
	// Replace default tester in main pkg with oru test implementation
	fetcher = tester
	crawlDomain("http://golang.org")

	// Check expected number of pages in the sitemap
	if len(sitemap) != 4 {
		t.Errorf("Expected 4 entries in the sitemap, got %d", len(sitemap))
	}

	// Check number of links for each page
	for url, links := range sitemap {
		if len(links) != len(tester[url]) {
			t.Errorf("expected %d entries for %s, got %d",
				len(tester[url]),
				url,
				len(links))
		}
	}
}
