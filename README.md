# Plato Test: Web-Crawler

This is basic a command-line tool written in Go. You must provide the first url, then the tool gets the page and look for links inside the document. All links will be saved, but only the links inside the initial url will be recursively fetched. The fecth function is async, so many request can be made simultaneously.

Very simple first usage:

`go run crawler.go -url='http://www.example.com/test' -output='example.json'`

## Syntax

Command parameters:

- url: initial url to fetch
- verbose: enable verbose mode for debugging
- resume: resume previous execution
- output: output filename (default is output.json)

## Output

Output is stored in a json file like this:

```
{
 "urls": [
  {
   "url": "http://chipiwini.com/plato",
   "completed": true,
   "error": 0,
   "links": [
    "http://chipiwini.com/plato/second_page.php",
    "javscript:void();",
    "http://chipiwini.COM/plato/tres.php",
    "http://chipiWini.com/plato/tres"
   ]
  },
  {
   "url": "http://chipiwini.com/plato/second_page.php",
   "completed": true,
   "error": 2,
   "links": null
  },
  {
   "url": "http://chipiwini.COM/plato/tres.php",
   "completed": true,
   "error": 0,
   "links": [
    "http://chipiwini.com/plato/four.php"
   ]
  },
  {
   "url": "http://chipiWini.com/plato/tres",
   "completed": true,
   "error": 0,
   "links": [
    "http://chipiwini.com/plato/four.php"
   ]
  },
  {
   "url": "http://chipiwini.com/plato/four.php",
   "completed": true,
   "error": 0,
   "links": [
    "http://chipiwini.com/plato",
    "http://chipiwini.com/plato/img.jpg"
   ]
  },
  {
   "url": "http://chipiwini.com/plato/img.jpg",
   "completed": true,
   "error": 3,
   "links": null
  }
 ]
}

```

The json file will store an array of unique urls, containing:

- **url**: its location
- **completed**: was the url fetched?
- **error**:
    - 0: no error
    - 1: error getting url (no such domain, etc)
    - 2: response code is not 200
    - 3: the document content type is not "text/html"
    - 4: there was an error parsing the html code
- **links**: array of links on the webpage

Note that links can be present in two or more urls, but urls are unique.

## When the tool stops

The tool will end in the following situations:

- All urls complete field are set.
- User press Ctrl+C. In this particular situation, the data will be saved in a file named *working.js*. In order to run again the tool, you have to set the -resume flag in command line parameters, or delete *working.js* file.

## What's next?

Possible improvements:

- Ignore links like "*javascript:func('blah');*".
- Consider relative links inside initial url (now only absolute paths are processed).
- Treat HTTPS and HTTP as the same, so if your initial url protocol is HTTP and it contains links inside the url but the server redirected us to HTTPS, walk to the HTTPS urls.
- Default output filename based on initial url, not just "output.json".
- Ask for output overwrite, and add parameter -force for not asking.
- Add maximum parameter, based on max urls to walk to, or maximum depth.
- Improve the resume feature.