/*
 * This module allows upload via mod_http_upload_external
 * Also see: https://modules.prosody.im/mod_http_upload_external.html
 */

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

/*
 * Configuration of this server
 */
type Config struct {
	Listenport   string
	UnixSocket   bool
	Secret       string
	Storedir     string
	UploadSubDir string
}

var conf Config
var versionString string = "0.0.0"

var ALLOWED_METHODS string = strings.Join(
    []string{
        http.MethodOptions,
        http.MethodHead,
        http.MethodGet,
        http.MethodPut,
    },
    ", ",
)

/*
 * Sets CORS headers
 */
func addCORSheaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", ALLOWED_METHODS)
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "7200")
}

/*
 * Request handler
 * Is activated when a clients requests the file, file information or an upload
 */
func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Println("Incoming request:", r.Method, r.URL.String())

	// Parse URL and args
	p := r.URL.Path

	a, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		log.Println("Failed to parse URL query params:", err)
		http.Error(w, "500 Internal Server Error", 500)
		return
	}

	subdir := path.Join("/", conf.UploadSubDir)
	fileStorePath := strings.TrimPrefix(p, subdir)
	if fileStorePath == "" || fileStorePath == "/" {
		log.Println("Empty request URL")
		http.Error(w, "403 Forbidden", 403)
		return
	} else if fileStorePath[0] == '/' {
		fileStorePath = fileStorePath[1:]
	}

	absFilename := filepath.Join(conf.Storedir, fileStorePath)

	// Add CORS headers
	addCORSheaders(w)

	if r.Method == http.MethodPut {
		// Check if MAC is attached to URL
		if a["token"] == nil {
			log.Println("Error: No HMAC attached to URL.")
			http.Error(w, "409 Conflict", 409)
			return
		}

		fmt.Println("MAC sent: ", a["token"][0])

		/*
		 * Check if the request is valid
		 */
        contentType := mime.TypeByExtension(filepath.Ext(fileStorePath))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		mac := hmac.New(sha256.New, []byte(conf.Secret))
		log.Println("fileStorePath:", fileStorePath)
		log.Println("ContentLength:", strconv.FormatInt(r.ContentLength, 10))
        log.Println("fileType:", contentType)
		mac.Write([]byte(fileStorePath + " " + strconv.FormatInt(r.ContentLength, 10) + contentType))
		macString := hex.EncodeToString(mac.Sum(nil))
        
        /*
		 * Check whether calculated (expected) MAC is the MAC that client send in "v" URL parameter
		 */
		if hmac.Equal([]byte(macString), []byte(a["token"][0])) {
			// Make sure the path exists
			err := os.MkdirAll(filepath.Dir(absFilename), os.ModePerm)
			if err != nil {
				log.Println("Could not make directories:", err)
				http.Error(w, "500 Internal Server Error", 500)
				return
			}

			file, err := os.OpenFile(absFilename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			defer file.Close()
			if err != nil {
				log.Println("Creating new file failed:", err)
				http.Error(w, "409 Conflict", 409)
				return
			}

			n, err := io.Copy(file, r.Body)
			if err != nil {
				log.Println("Writing to new file failed:", err)
				http.Error(w, "500 Internal Server Error", 500)
				return
			}

			log.Println("Successfully written", n, "bytes to file", fileStorePath)
			w.WriteHeader(http.StatusCreated)
			return
		} else {
			log.Println("Invalid MAC.")
            log.Println(macString, " is different than ", a["token"][0])
			http.Error(w, "403 Forbidden", 403)
			return
		}
	} else if r.Method == http.MethodHead || r.Method == http.MethodGet {
		fileinfo, err := os.Stat(absFilename)
		if err != nil {
			log.Println("Getting file information failed:", err)
			http.Error(w, "404 Not Found", 404)
			return
		} else if fileinfo.IsDir() {
			log.Println("Directory listing forbidden!")
			http.Error(w, "403 Forbidden", 403)
			return
		}

		/*
		 * Find out the content type to sent correct header. There is a Go function for retrieving the
		 * MIME content type, but this does not work with encrypted files (=> OMEMO). Therefore we're just
		 * relying on file extensions.
		 */
		contentType := mime.TypeByExtension(filepath.Ext(fileStorePath))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)

		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.FormatInt(fileinfo.Size(), 10))
		} else {
			http.ServeFile(w, r, absFilename)
		}

		return
	} else if r.Method == http.MethodOptions {
		w.Header().Set("Allow", ALLOWED_METHODS)
		return
	} else {
		log.Println("Invalid method", r.Method, "for access to ", conf.UploadSubDir)
		http.Error(w, "405 Method Not Allowed", 405)
		return
	}
}

func readConfig(configfilename string, conf *Config) error {
	log.Println("Reading configuration ...")

	configdata, err := ioutil.ReadFile(configfilename)
	if err != nil {
		log.Fatal("Configuration file config.toml cannot be read:", err, "...Exiting.")
		return err
	}

	if _, err := toml.Decode(string(configdata), conf); err != nil {
		log.Fatal("Config file config.toml is invalid:", err)
		return err
	}

	return nil
}

/*
 * Main function
 */
func main() {
	var configFile string
	var proto string

	/*
	 * Read startup arguments
	 */
	flag.StringVar(&configFile, "config", "./config.toml", "Path to configuration file \"config.toml\".")
	flag.Parse()

	if !flag.Parsed() {
		log.Fatalln("Could not parse flags")
	}


	/*
	 * Read config file
	 */
	err := readConfig(configFile, &conf)
	if err != nil {
		log.Fatalln("There was an error while reading the configuration file:", err)
	}

	if conf.UnixSocket {
		proto = "unix"
	} else {
		proto = "tcp"
	}

	/*
	 * Start HTTP server
	 */
	log.Println("Starting Prosody-Filer", versionString, "...")
	listener, err := net.Listen(proto, conf.Listenport)
	if err != nil {
		log.Fatalln("Could not open listening socket:", err)
	}

	subpath := path.Join("/", conf.UploadSubDir)
	subpath = strings.TrimRight(subpath, "/")
	subpath += "/"
	http.HandleFunc(subpath, handleRequest)
	log.Printf("Server started on port %s. Waiting for requests.\n", conf.Listenport)
	http.Serve(listener, nil)
}
