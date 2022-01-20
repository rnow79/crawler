package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/net/html"
)

// Command-Line options
var verbose *bool      // Verbose bit
var resume *bool       // Resume last execution
var initialUrl *string // Initial URL to fetch
var outputFile *string // Name of the output file

// Url type
type Url struct {
	URL       string   `json:"url"`       // Stores the URL
	Completed bool     `json:"completed"` // Tells us if the URL is processed
	Error     int      `json:"error"`     // There was some error fetching the URL
	Links     []string `json:"links"`     // Array of links
}

// Urls type
type Urls struct {
	URLS []Url `json:"urls"`
}

// Variables
const workingFile string = "working.json"

var urls Urls // Main struct for storing urls

func newUrl(u string) *Url {
	retUrl := Url{URL: u}
	return &retUrl
}

// Prepare a channel and run anonymous asyncronous function. It will be waiting for the os interrupt
// then saves the links database.
func interruptHandler() {
	intChan := make(chan os.Signal)
	signal.Notify(intChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-intChan
		// SIGTERM Received, save our progress and exit!

		log.Println("Program Stopped by user")
		// Save to disk
		file, _ := json.MarshalIndent(urls, "", " ")
		_ = ioutil.WriteFile(workingFile, file, 0644)
		if *verbose {
			log.Println("Saving working file")
			log.Printf("Progress saved in working.json (%d urls)\n", len(urls.URLS))
		}

		os.Exit(0)
	}()
}

func completeUrl(id int, err int) {
	logLine(id, "process ended with exit code %d.", err)
	urls.URLS[id].Completed = true
	urls.URLS[id].Error = err
}
func logLine(id int, format string, v ...interface{}) {
	if *verbose {
		log.Println(fmt.Sprintf(fmt.Sprintf("Process id %d - %s", id, format), v...))
	}
}

func validURL(id int, u string) bool {
	_, err := url.ParseRequestURI(u)
	if err != nil {
		logLine(id, "url %s is not valid", u)
		return false
	}
	if !strings.HasPrefix(strings.ToLower(u), strings.ToLower(*initialUrl)) {
		logLine(id, "url %s is not a child of the initial url", u)
		return false
	}

	for url := range urls.URLS {
		if strings.EqualFold(urls.URLS[url].URL, u) {
			logLine(id, "url %s already in database", u)
			return false
		}
	}
	return true
}

func linkExists(link string, links []string) bool {
	for index := range links {
		if strings.EqualFold(link, links[index]) {
			return true
		}
	}
	return false
}

func fetchUrl(id int, u string) {
	action := "fetching"
	if *resume {
		action = "resuming "
	}
	logLine(id, "%s url %s", action, u)
	response, err := http.Get(u)
	if err != nil {
		completeUrl(id, 1) // Error 1 => Probably network related (dns, unable to connect...)
		return
	}
	if response.StatusCode != 200 {
		completeUrl(id, 2) // Error 2 => Response code from http server is not 200
		return
	}
	// Check Content Type
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/html") {
		completeUrl(id, 3) // Error 3 => The document fetched content is not HTML
		return
	}
	body, err := html.Parse(response.Body)
	response.Body.Close()
	if err != nil {
		completeUrl(id, 4) // Error 3 => Unable to parse HTML document
		return
	}
	var links []string
	visitNode := func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key != "href" {
					continue
				}
				if len(a.Val) == 0 {
					continue // Ignoring empty links
				}
				// Check for duplicate links (in the current URL)
				if linkExists(a.Val, links) {
					logLine(id, "ignoring duplicate link %s", a.Val)
					continue
				}
				links = append(links, a.Val)
			}
		}
	}
	forEachNode(body, visitNode, nil)
	for link := range links {
		// Add link (even if malformed) to the link list
		logLine(id, "found link %s", links[link])
		urls.URLS[id].Links = append(urls.URLS[id].Links, links[link])
		// Now we have to determine if the link is a valid URL and starts like the initial one
		if validURL(id, links[link]) {
			logLine(id, "adding url %s for being processed", links[link])
			urls.URLS = append(urls.URLS, *newUrl(links[link]))
			go fetchUrl(len(urls.URLS)-1, links[link])
		}
	}
	completeUrl(id, 0)
}

func forEachNode(n *html.Node, pre, post func(n *html.Node)) {
	if pre != nil {
		pre(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		forEachNode(c, pre, post)
	}
	if post != nil {
		post(n)
	}
}

func allDone() bool {
	for u := range urls.URLS {
		if !urls.URLS[u].Completed {
			return false
		}
	}
	return true // All URLs have been processed, beer time!
}

func main() {
	// Command-Line options
	verbose = flag.Bool("verbose", false, "Enable verbose mode")
	resume = flag.Bool("resume", false, "Resume last execution")
	initialUrl = flag.String("url", "", "Initial URL to fetch")
	outputFile = flag.String("output", "output.json", "Output filename")

	flag.Parse()

	if strings.EqualFold(*outputFile, workingFile) {
		log.Fatal("Please use other output filename, since ", workingFile, " is reserved")
	}

	if len(*initialUrl) == 0 && !*resume {
		log.Fatal("No initial url provided, only allowed when resuming")
	}

	if *verbose {
		log.Println("Resume:", *resume)
		log.Println("InitialURL:", *initialUrl)
		log.Println("OutputFile:", *outputFile)
	}

	// Check if there is previous data
	_, err := os.Stat(workingFile)
	if !errors.Is(err, os.ErrNotExist) {
		if !*resume {
			log.Println("FATAL: Working file found!")
			log.Fatal("Use -resume flag for resume the execution, or remove it and execute again!")
		}
		jsonFile, _ := ioutil.ReadFile(workingFile)
		_ = json.Unmarshal([]byte(jsonFile), &urls)
		if err != nil {
			log.Fatal("Error parsing", workingFile) // stop program
		}
		if *verbose {
			log.Println("Working file loaded")
		}
		if len(urls.URLS) == 0 {
			log.Fatal("Empty or malformed working.json, please check or delete it", workingFile) // stop program
		}
		*initialUrl = urls.URLS[0].URL
	} else {
		// First URL insertion
		first := newUrl(*initialUrl)
		urls.URLS = append(urls.URLS, *first)
	}

	// Ctrl+C handler
	interruptHandler()

	// Process the incomplete urls (if not resuming, It will be just the first)
	for index := range urls.URLS {
		if !urls.URLS[index].Completed {
			go fetchUrl(index, urls.URLS[index].URL)
		}
	}
	for !allDone() {
	}

	// Save to disk
	if len(urls.URLS) > 0 {
		file, _ := json.MarshalIndent(urls, "", " ")
		_ = ioutil.WriteFile(*outputFile, file, 0644)
		_ = os.Remove(workingFile)
		if *verbose {
			log.Println("All done!")
			log.Printf("File %s saved with %d URLS.\n", *outputFile, len(urls.URLS))
		}
	}
}
