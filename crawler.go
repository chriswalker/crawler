package main

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

// Fetcher defines a simple interface for getting a list of anchors and images
// found within a supplied URL.
// It is defined here as an interface so we can provide test implementations.
type Fetcher interface {
	// Fetch returns a list of links in the supplied URL
	Get(url string) (links []string, err error)
}

// Define our real fetcher.
type URLFetcher struct{}

// Get retrieves a list of links or images found within the supplied URL.
func (u URLFetcher) Get(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("getting: %s: %s", url, resp.Status)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing %s as HTML: %v", url, err)
	}
	resp.Body.Close()

	var (
		links     []string
		checkNode func(n *html.Node)
	)

	// Process the current node in the tree
	checkNode = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key != "href" {
					continue
				}
				link, err := resp.Request.URL.Parse(a.Val)
				if err != nil {
					continue // Ignore bad URLs for moment
				}
				links = append(links, link.String())
			}
		}
		if n.Type == html.ElementNode && n.Data == "img" {
			for _, a := range n.Attr {
				if a.Key != "src" {
					continue
				}
				link, err := resp.Request.URL.Parse(a.Val)
				if err != nil {
					continue // As above
				}
				links = append(links, link.String())
			}
		}

		// Visit siblings
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			checkNode(c)
		}
	}

	// Recusively visit nodes
	checkNode(doc)

	return links, nil
}

var (
	// The sitemap, mapping URLs to lists of links under that URL
	sitemap map[string][]string
	// The implementation of Fetcher that gets a page and extracts the links in it
	fetcher Fetcher = new(URLFetcher)
	// Counter channel for limiting concurrent requests
	counter chan struct{}
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Missing URL to crawl")
		os.Exit(1)
	}

	crawlDomain(os.Args[1])

	fmt.Printf("Found %d pages from %s\n", len(sitemap), os.Args[1])

	// Map isn't sorted, so get keys and sort them for ordered output
	var sortedUrls []string
	for url := range sitemap {
		sortedUrls = append(sortedUrls, url)
	}
	sort.Strings(sortedUrls)
	for _, url := range sortedUrls {
		fmt.Println(url)
		for _, link := range sitemap[url] {
			fmt.Printf("\t%s\n", link)
		}
	}
}

// crawlDomain takes the supplied URL and gets the links on the page; goroutines
// are created for each unseen link to further build out the sitemap
func crawlDomain(url string) {
	// Define number of parallel requests allowed here
	counter = make(chan struct{}, 20)
	// List of links we have already visitied
	visited := make(map[string]bool)
	// The list of links by page
	sitemap = make(map[string][]string)
	// Mutex to protect access to sitemap
	var mutex sync.Mutex

	// This channel holds the list of URLs we know about, but haven't yet crawled
	pages := make(chan []string)
	n := 1 // Number of pages still to crawl in the channel
	go func() {
		pages <- []string{url}
	}()

	// Process lists of URLs. If n gets to 0, we have no more running routines and
	// hence no more pages to visit, at which point we're done
	for ; n > 0; n-- {
		links := <-pages
		for _, link := range links {
			if !visited[link] && !isImageLink(link) && isSameDomain(url, link) {
				visited[link] = true
				n++
				go func(link string) {
					list := getLinks(link)

					mutex.Lock()
					sitemap[link] = list
					mutex.Unlock()

					pages <- list
				}(link)
			}
		}
	}
}

// getLinks retrieves the HTML for the supplied URL, and extracts all anchor
// links and images on the page. If we cannot push a counter onto the counters
// channel, that means we have the maximum number of goroutines running, and
// so will block until the send can occur.
func getLinks(url string) []string {
	counter <- struct{}{}
	links, err := fetcher.Get(url)
	<-counter
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return links
}

// isImageLink determines whether the supplied URL is for an image (.png, .jpg,
// .gif etc). Returns true if it's an image, false otherwise
func isImageLink(url string) bool {
	exp := regexp.MustCompile(`([^\s]+(\.(?i)(jpg|png|gif|bmp|tiff))$)`)
	if exp.MatchString(url) {
		return true
	}
	return false
}

// isSameDomain checks the prefix of the supplied URL against the initial URL
// passed to the crawler; returns true if they match, false otherwise
func isSameDomain(domain, url string) bool {
	if strings.HasPrefix(url, domain) {
		return true
	}
	return false
}
