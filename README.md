# Plato Test: Web-Crawler

This is basic a command-line tool written in Go. You must provide the first url, then the tool gets the page and look for links inside the document. All links will be saved, but only the links inside the initial url will be recursively fetched. The fecth function is async, so many request can be made simultaneously.

Very simple first usage:

`go run crawler.go -url=http://www.example.com/test -output=example.json`

## Syntax

Command parameters:

- url: initial url to fetch
- verbose: enable verbose mode for debugging
- resume: resume previous execution
- output: output filename (default is output.json)

## What's next?

Possible improvements:

- Ignore links like "*javascript:func('blah');*".
- Consider relative links inside initial url (now only absolute paths are processed).
- Treat HTTPS and HTTP as the same, so if your initial url protocol is HTTP and it contains links inside the url but the server redirected us to HTTPS, walk to the HTTPS urls.
- Default output filename based on initial url, not just "output.json".
- Ask for output overwrite, and add parameter -force for not asking.
- Add maximum parameter, based on max urls to walk to, or maximum depth.
