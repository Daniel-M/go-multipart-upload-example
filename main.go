package main

import (
	//"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

var (
	uploadSucess = `<html>
	<head>
		<script src="/socket.io/socket.io.js"></script>
	</head>
	<body>
	<h2>Upload completed!</h2>
G	<a href="http://localhost:8080">Go back</a>
	</body>
	<script>
		const socket = io('http://localhost');
	</script>
</html>`
	uploadError = `<html>
	<body>
	<h2>Errors occurred during upload</h2>
	<a href="http://localhost:8080">Go back</a>
	</body>
</html>`
	uploadInvalid = `<html>
	<body>
	<h2>Invalid file type</h2>
	<a href="http://localhost:8080">Go back</a>
	</body>
</html>`
)

var (
	// UploadChunkSize Chunk size for the upload in bytes
	UploadChunkSize = 4096

	// ValidMimeTypes store the valid mime types for the uploaded files
	ValidMimeTypes = []string{"image/tiff"}
)

func isValidMimeType(s string) bool {
	for _, v := range ValidMimeTypes {
		if v == s {
			return true
		}
	}
	return false
}
func UploadServerError(message string, w http.ResponseWriter, err error) {
	log.Println("Could not process the upload:", message)
	log.Println(err)
	b, _ := json.Marshal(map[string]interface{}{"message": "Could not process the upload request"})
	w.WriteHeader(http.StatusInternalServerError)
	io.WriteString(w, string(b))
	return
}

func UploadInvalidMimeType(mimetype string, w http.ResponseWriter) {
	message := "Invalid mime-type for uploaded file, " + mimetype
	log.Println(message)
	b, _ := json.Marshal(map[string]interface{}{"message": message})
	w.WriteHeader(http.StatusBadRequest)
	io.WriteString(w, string(b))
	return
}

var handlers = map[string]func(http.ResponseWriter, *http.Request){
	"POST": handlePost,
}

var dashhandlers = map[string]func(http.ResponseWriter, *http.Request){
	"GET": handleGet,
}

// getHandlers map regexp with request handlers
var getHandlers = map[*regexp.Regexp]func(*regexp.Regexp, http.ResponseWriter, *http.Request){
	regexp.MustCompile(`/`):       getDashboard,
	regexp.MustCompile(`/upload`): getDashboard,
}

// handleGet runs the appropriate handler according to the route and the
// registered regular expressions on the postHandler map
func handleGet(w http.ResponseWriter, req *http.Request) {
	for re, fx := range getHandlers {
		urlPath := strings.TrimRight(req.URL.Path, "/")
		match := re.MatchString(urlPath)

		//pathSlash := strings.Count(urlPath, "/")
		//reSlash := strings.Count(re.String(), "/")

		//log.Println("urlPath", urlPath)
		//log.Println("match", match)
		//log.Println("pathSlash", pathSlash)
		//log.Println("reSlash", reSlash)

		//log.Println("match && pathSlash == reSlash")
		//log.Println(match && pathSlash == reSlash)

		//if match && pathSlash == reSlash {
		if match {
			log.Println("Serving under", re.String())
			fx(re, w, req)
			return
		}
	}
	http.NotFound(w, req)
}

// getDashboard serves the viewer interface
func getDashboard(re *regexp.Regexp, w http.ResponseWriter, req *http.Request) {
	log.Println("Getting dashboard...")
	http.FileServer(http.Dir("./static")).ServeHTTP(w, req)
}

// postHandlers map regexp with request handlers
var postHandlers = map[*regexp.Regexp]func(*regexp.Regexp, http.ResponseWriter, *http.Request){
	regexp.MustCompile(`/upload`): uploadTiles,
}

// uploadTiles serves the endpoint to upload tiles
func uploadTiles(re *regexp.Regexp, w http.ResponseWriter, req *http.Request) {
	log.Println("Got multipart upload, reading parts...")

	readParts(w, req)
}

// handlePost runs the appropriate handler according to the route and the
// registered regular expressions on the postHandler map
func handlePost(w http.ResponseWriter, req *http.Request) {
	for re, fx := range postHandlers {
		urlPath := strings.TrimRight(req.URL.Path, "/")
		match := re.MatchString(urlPath)

		pathSlash := strings.Count(urlPath, "/")
		reSlash := strings.Count(re.String(), "/")

		//log.Println("urlPath", urlPath)
		//log.Println("match", match)
		//log.Println("pathSlash", pathSlash)
		//log.Println("reSlash", reSlash)

		//log.Println("match && pathSlash == reSlash")
		//log.Println(match && pathSlash == reSlash)

		if match && pathSlash == reSlash {
			//if match {
			log.Println("Serving under", re.String())
			fx(re, w, req)
			return
		}
	}
	http.NotFound(w, req)
}

func processTheFile(f *os.File) {
	defer func() {
		log.Printf("Deleting the file %s", f.Name())
		os.Remove(f.Name())
	}()

	f, err := os.Open(f.Name())
	if err != nil {
		log.Printf("Could not open the file\n")
		log.Fatal(err)
	}

	defer f.Close()

	log.Printf("Opening the file %s", f.Name())

	if n, err := f.Seek(0, 0); err != nil || n != 0 {
		log.Printf("unable to seek to beginning of file '%s'", f.Name())
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Printf("unable to hash '%s': %s", f.Name(), err.Error())
	}
	log.Printf("SHA256 sum of '%s': %x", f.Name(), h.Sum(nil))

	out, err := os.Create("uploads/" + strings.Split(f.Name(), "/")[2])
	if err != nil {
		log.Println("Unable to create file")
		log.Println(err)
		return
	}

	if n, err := f.Seek(0, 0); err != nil || n != 0 {
		log.Printf("unable to seek to beginning of file '%s'", f.Name())
	}
	_, err = io.Copy(out, f)
	if err != nil {
		log.Println("Unable to copy the contents of file")
		log.Println(err)
	}

	defer out.Close()

	log.Printf("I'm outta processTheFile")
	return
}

func readParts(w http.ResponseWriter, r *http.Request) {
	// define some variables used throughout the function
	// n: for keeping track of bytes read and written
	// err: for storing errors that need checking

	log.Println("File Upload Endpoint Hit")
	multipartReader, err := r.MultipartReader()
	if err != nil {
		message := "Error while opening multipart reader: " + err.Error()
		UploadServerError(message, w, err)
		return
	}

	// buffer to be used for reading bytes from files
	chunk := make([]byte, UploadChunkSize)
	// variables used in this loop only
	// tempfile: filehandler for the temporary file
	// filesize: how many bytes where written to the tempfile
	// uploaded: boolean to flip when the end of a part is reached
	var tempfile *os.File
	var filesize int

	// Get the first part to extract the metadata
	/* part, mpErr := multipartReader.NextPart()*/
	//if mpErr != nil {
	//message := "Error while opening multipart reader: " + err.Error()
	//UploadServerError(message, w, err)
	//return
	/*}*/

	// iterate over all parts until EOF
	for {
		// Get the next part
		part, mpErr := multipartReader.NextPart()
		if mpErr != nil && mpErr != io.EOF {
			message := "Error while opening multipart reader: " + err.Error()
			log.Println(message)
			break
		} else if mpErr == io.EOF {
			log.Println("No more parts to read")
			break
		}

		switch part.FormName() {

		case "layer":
			bodyBytes := make([]byte, UploadChunkSize)

			n, err := part.Read(bodyBytes)
			if err != nil && err != io.EOF {
				log.Println("Could not read the layer contents")
				log.Println(err)
				continue
			}

			log.Println("Read", n, "bytes.")
			log.Println("bodyBytes:")
			log.Println(string(bodyBytes))

			var body map[string]interface{}
			err = json.Unmarshal(bodyBytes[:n], &body)
			if err != nil {
				log.Println("Could not unmarshal layer contents")
				log.Println(err)
				continue
			}
			log.Println("Layer:")
			log.Println(body)
			continue

		case "file":
			contentType := part.Header.Get("Content-Type")
			filename := part.FileName()
			// at this point the filename and the mimetype is known
			log.Printf("Uploaded filename: %s", filename)
			log.Printf("Uploaded mimetype: %s", contentType)

			if !isValidMimeType(contentType) {
				UploadInvalidMimeType(contentType, w)
				return
			}

			// Create the temporary file to store the uploaded contents
			tempfile, err = ioutil.TempFile(os.TempDir(), "upload-*.tif")
			if err != nil {
				message := "Hit error while creating temp file: " + err.Error()
				UploadServerError(message, w, err)
				return
			}
			defer tempfile.Close() // Defer closing the file
			//defer os.Remove(tempfile.Name())

			// here the temporary filename is known
			log.Printf("Temporary filename: %s\n", tempfile.Name())

			// Read all chunks in the part
			// until EOF
			for {
				n, readErr := part.Read(chunk)

				if readErr != nil && readErr != io.EOF {
				} else if readErr == io.EOF {
					break
				}

				// Write to the temporary file
				if n, err = tempfile.Write(chunk[:n]); err != nil {
					message := "Hit error while reading chunk: " + err.Error()
					UploadServerError(message, w, err)
					return
				}

				filesize += n
			}

		default:
			continue

		}

	}

	// once uploaded something can be done with the file, the last defer
	// statement will remove the file after the function returns so any
	// errors during upload won't hit this, but at least the tempfile is
	// cleaned up
	if tempfile != nil {
		processTheFile(tempfile)
	}

	log.Printf("Uploaded filesize: %d bytes", filesize)
}

// Upload is the main controller of requests to the /tiles and /viewer... endpoint
// it dispatches handlers according to the HTTP verb and the registered
// handlers in handlers map. Recall that handlers map associates http verbs
// with verb-handler (getHandlers, putHandlers, deleteHandlers, postHandlers)
// that also maps regular expressions with request handlers
func Upload() func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if fx, ok := handlers[req.Method]; ok {
			log.Println("Incoming request")
			log.Println(req)
			fx(w, req)
			return
		}
		http.NotFound(w, req)
	}
}

// View is the main controller of requests to the /tiles and /viewer... endpoint
// it dispatches handlers according to the HTTP verb and the registered
// handlers in handlers map. Recall that handlers map associates http verbs
// with verb-handler (getHandlers, putHandlers, deleteHandlers, postHandlers)
// that also maps regular expressions with request handlers
func View() func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if fx, ok := dashhandlers[req.Method]; ok {
			log.Println("Incoming request")
			log.Println(req)
			fx(w, req)
			return
		}
		http.NotFound(w, req)
	}
}

func main() {
	log.Println("Upload server running at :8080")
	//http.HandleFunc("/upload", readParts)
	http.HandleFunc("/upload", Upload())
	http.HandleFunc("/", http.FileServer(http.Dir("./static")).ServeHTTP)
	log.Fatalf("Exited: %s", http.ListenAndServe(":8080", nil))
}
