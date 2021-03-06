// This is a basic command-line tool written in Go. You must provide the first url, then the tool gets the page
// and look for links inside the document. All links will be saved, but only the links inside the initial url
// will be recursively fetched. The fecth function is async, so many request can be made simultaneously.
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
	"time"

	"golang.org/x/net/html"
)

// Name of the file where we'll save our progess if the user interrupts the execution.
const workingFile string = "working.json"

// Command variable: Verbose bit.
var verbose bool

// Command variable: Resume last execution.
var resume bool

// Command variable: Initial URL to fetch.
var initialUrl string

// Command variable: Name of the output file.
var outputFile string

// Main struct for storing urls.
var urls Urls

// Numer of goroutines running at the same time.
const MaxRoutines int = 2

// Url type.
type Url struct {
	URL       string   `json:"url"`       // Stores the URL.
	Completed bool     `json:"completed"` // Tells us if the URL is processed.
	Error     int      `json:"error"`     // There was some error fetching the URL.
	Links     []string `json:"links"`     // Array of links.
}

// Urls type.
type Urls struct {
	URLS []Url `json:"urls"`
}

// Returns a new empty Url object.
func newUrl(u string) *Url {
	retUrl := Url{URL: u}
	return &retUrl
}

// Prepare a channel and run anonymous asyncronous function. It will be waiting for the os interrupt,
// then saves the link database.
func interruptHandler() {
	intChan := make(chan os.Signal)
	signal.Notify(intChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		// Wait for os to nofity us using the interrupt channel.
		<-intChan
		// SIGTERM Received, save our progress and exit!
		log.Println("Program Stopped by user")
		// Save to disk.
		file, err := json.MarshalIndent(urls, "", " ")
		// Since we Marshal an Url struct, it's really difficult to have here an error,
		// anyway let's check it.
		if err != nil {
			log.Fatal("Error while parsing urls:", err)
		}
		// Write to file.
		err = ioutil.WriteFile(workingFile, file, 0644)
		if err != nil {
			log.Fatal("Error writting file to disk:", err)
		}
		if verbose {
			log.Println("Saving working file")
			log.Printf("Progress saved in working.json (%d urls)\n", len(urls.URLS))
		}
		// Exit application.
		os.Exit(0)
	}()
}

// Mark an URL as completed, and save the error code.
func completeUrl(id int, err int, main chan int, end chan int) {
	logLine(id, "process ended with exit code %d.", err)
	urls.URLS[id].Completed = true
	urls.URLS[id].Error = err
	// Tell the channel to start next goroutine in the buffer
	main <- 1
	// Give some time for next routine to start
	time.Sleep(1 * time.Second)
	// If all data is processed, end
	if allDone() {
		end <- 1
	}
}

// log.Println if verbose bit is set.
func logLine(id int, format string, v ...interface{}) {
	if verbose {
		log.Println(fmt.Sprintf(fmt.Sprintf("Process id %d - %s", id, format), v...))
	}
}

// Whenever a link is found, we pass the url to this funcion, in order
// to evalute if we hav to execute a new process walking to it.
func validURL(id int, u string) bool {
	// Check basic url parsing provided by net/url package.
	_, err := url.ParseRequestURI(u)
	if err != nil {
		logLine(id, "url %s is not valid", u)
		return false
	}
	// Check if the url starts like the initial url.
	if !strings.HasPrefix(strings.ToLower(u), strings.ToLower(initialUrl)) {
		logLine(id, "url %s is not a child of the initial url", u)
		return false
	}
	// If url starts like initialUrl, now we have to check if next rune
	// of url, skipping initialURL lenght, is / or ?, So if initial us is
	// http://example.com/test, then:
	//  - http://example.com/test/page.htm will return true
	//  - http://example.com/test?id=10 will return true
	//  - http://example.com/test10 will return false
	fmt.Printf("[%s]", u)
	if len(u) > len(initialUrl) {
		nextrune := u[len(initialUrl)]
		if nextrune == '/' || nextrune == '?' {
			return true
		}
	}
	// Finally, check if the url is not already in our database.
	for url := range urls.URLS {
		if strings.EqualFold(urls.URLS[url].URL, u) {
			logLine(id, "url %s already in database", u)
			return false
		}
	}
	return true
}

// Returns true if the string "link" exists in the array "links".
func linkExists(link string, links []string) bool {
	for index := range links {
		if strings.EqualFold(link, links[index]) {
			return true
		}
	}
	return false
}

// This is the funcion we will fire for each url, it will fetch the url and parse the response.
func fetchUrl(id int, u string, mainChannel chan int, endChannel chan int) {
	//	time.Sleep(1 * time.Second)
	// Check out if we're in resume mode, just for logging purpuses.
	action := "fetching"
	if resume {
		action = "resuming "
	}
	logLine(id, "%s url %s", action, u)
	// Fetch the url.
	response, err := http.Get(u)
	if err != nil {
		// Error 1 => Probably network related (dns, unable to connect...).
		completeUrl(id, 1, mainChannel, endChannel)
		return
	}
	// Error 2 => Response code from http server is not 200.
	if response.StatusCode != 200 {
		completeUrl(id, 2, mainChannel, endChannel)
		return
	}
	// Error 3 => The document fetched content is not HTML.
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/html") {
		completeUrl(id, 3, mainChannel, endChannel)
		return
	}
	// Error 3 => Error while parsing HTML document.
	body, err := html.Parse(response.Body)
	response.Body.Close()
	if err != nil {
		completeUrl(id, 4, mainChannel, endChannel)
		return
	}
	// Extract links from document.
	var links []string
	// This function will decide what to do with an html tag.
	visitNode := func(n *html.Node) {
		// We only want <a> html tags.
		if n.Type == html.ElementNode && n.Data != "a" {
			return
		}
		// Let's inspect all attributes in the tag.
		for _, a := range n.Attr {
			// Check if attributes is "href".
			if a.Key != "href" {
				continue
			}
			// Ignoring empty links.
			if len(a.Val) == 0 {
				continue
			}
			// Ignoring duplicate links.
			if linkExists(a.Val, links) {
				logLine(id, "ignoring duplicate link %s", a.Val)
				continue
			}
			// Add links to the url links array.
			links = append(links, a.Val)
		}

	}
	// Parse document and pass our tag parser.
	forEachNode(body, visitNode, nil)
	for link := range links {
		// Add link (even if malformed) to the link list.
		logLine(id, "found link %s", links[link])
		urls.URLS[id].Links = append(urls.URLS[id].Links, links[link])
		// If it's a valid url, add it to the database and start a walk process.
		if validURL(id, links[link]) {
			logLine(id, "adding url %s for being processed", links[link])
			urls.URLS = append(urls.URLS, *newUrl(links[link]))
			go fetchUrl(len(urls.URLS)-1, links[link], mainChannel, endChannel)
		}
	}
	// Document parsed and all link extracted and evaluated. Mark as completed.
	completeUrl(id, 0, mainChannel, endChannel)
	if allDone() {
		endChannel <- 1
	}
}

// Recursive function for extract html tags.
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

// See if all urls in the database are completed.
func allDone() bool {
	for _, u := range urls.URLS {
		if !u.Completed {
			return false
		}
	}
	// All URLs have been processed, beer time!
	return true
}

// Main function.
func main() {
	// Command-Line options.
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose mode")
	flag.BoolVar(&resume, "resume", false, "Resume last execution")
	flag.StringVar(&initialUrl, "url", "", "Initial URL to fetch")
	flag.StringVar(&outputFile, "output", "output.json", "Output filename")
	flag.Parse()
	// Output file cannot be named like working file.
	if strings.EqualFold(outputFile, workingFile) {
		log.Fatal("Please use other output filename, since ", workingFile, " is reserved")
	}
	// Check if initial url is present (and not resuming).
	if len(initialUrl) == 0 && !resume {
		log.Fatal("No initial url provided, only allowed when resuming")
	}
	// Print some variables.
	if verbose {
		log.Println("Resume:", resume)
		log.Println("InitialURL:", initialUrl)
		log.Println("OutputFile:", outputFile)
	}
	// Check if there is previous data.
	_, err := os.Stat(workingFile)
	if !errors.Is(err, os.ErrNotExist) {
		if !resume {
			log.Println("FATAL: Working file found!")
			log.Fatal("Use -resume flag for resume the execution, or remove it and execute again!")
		}
		jsonFile, _ := ioutil.ReadFile(workingFile)
		_ = json.Unmarshal([]byte(jsonFile), &urls)
		if err != nil {
			log.Fatal("Error parsing", workingFile) // Stop program.
		}
		if verbose {
			log.Println("Working file loaded")
		}
		if len(urls.URLS) == 0 {
			log.Fatal("Empty or malformed working.json, please check or delete it", workingFile) // Stop program.
		}
		initialUrl = urls.URLS[0].URL
	} else {
		// First URL insertion.
		first := newUrl(initialUrl)
		urls.URLS = append(urls.URLS, *first)
	}
	// Register Ctrl+C handler.
	interruptHandler()
	// mainChannel is a buffered channel, for fetching urls
	mainChannel := make(chan int, MaxRoutines)
	// endChannel will waint until all is processed
	endChannel := make(chan int)
	// Process the incomplete urls (if not resuming, it will be just the first).
	for i, u := range urls.URLS {
		if !u.Completed {
			go fetchUrl(i, u.URL, mainChannel, endChannel)
		}
	}
	// Wait for goroutines write to endChannel
	<-endChannel
	// Save to disk.
	file, _ := json.MarshalIndent(urls, "", " ")
	_ = ioutil.WriteFile(outputFile, file, 0644)
	_ = os.Remove(workingFile)
	if verbose {
		log.Println("All done!")
		log.Printf("File %s saved with %d URLS.\n", outputFile, len(urls.URLS))
	}
}
