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
	"net/http"
	"net/url"
	"os"
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
	Secret       string
	Storedir     string
	UploadSubDir string
}

var conf Config

/*
 * Sets CORS headers
 */
func addCORSheaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, HEAD, GET, PUT")
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
	u, err := url.Parse(r.URL.String())
	if err != nil {
		log.Println("Failed to parse URL:", err)
	}

	a, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		log.Println("Failed to parse URL query params:", err)
	}

	fileStorePath := strings.TrimPrefix(u.Path, "/"+conf.UploadSubDir)

	// Add CORS headers
	addCORSheaders(w)

	if r.Method == "PUT" {
		// Check if MAC is attached to URL
		if a["v"] == nil {
			log.Println("Error: No HMAC attached to URL.")
			http.Error(w, "409 Conflict", 409)
			return
		}

		fmt.Println("MAC sent: ", a["v"][0])

		/*
		 * Check if the request is valid
		 */
		mac := hmac.New(sha256.New, []byte(conf.Secret))
		log.Println("fileStorePath:", fileStorePath)
		log.Println("ContentLength:", strconv.FormatInt(r.ContentLength, 10))
		mac.Write([]byte(fileStorePath + " " + strconv.FormatInt(r.ContentLength, 10)))
		macString := hex.EncodeToString(mac.Sum(nil))

		/*
		 * Check whether calculated (expected) MAC is the MAC that client send in "v" URL parameter
		 */
		if hmac.Equal([]byte(macString), []byte(a["v"][0])) {
			// Make sure the path exists
			os.MkdirAll(filepath.Dir(conf.Storedir+fileStorePath), os.ModePerm)

			file, err := os.OpenFile(conf.Storedir+fileStorePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
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
		} else {
			log.Println("Invalid MAC.")
			http.Error(w, "403 Forbidden", 403)
			return
		}
	} else if r.Method == "HEAD" {
		fileinfo, err := os.Stat(conf.Storedir + fileStorePath)
		if err != nil {
			log.Println("Getting file information failed:", err)
			http.Error(w, "404 Not Found", 404)
			return
		}

		/*
		 * Find out the content type to sent correct header. There is a Go function for retrieving the
		 * MIME content type, but this does not work with encrypted files (=> OMEMO). Therefore we're just
		 * relying on file extensions.
		 */
		contentType := mime.TypeByExtension(filepath.Ext(fileStorePath))
		w.Header().Set("Content-Length", strconv.FormatInt(fileinfo.Size(), 10))
		w.Header().Set("Content-Type", contentType)
	} else if r.Method == "GET" {
		contentType := mime.TypeByExtension(filepath.Ext(fileStorePath))
		if f, err := os.Stat(conf.Storedir + fileStorePath); err != nil || f.IsDir() {
			log.Println("Directory listing forbidden!")
			http.Error(w, "403 Forbidden", 403)
			return
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		http.ServeFile(w, r, conf.Storedir+fileStorePath)
		w.Header().Set("Content-Type", contentType)
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
	/*
	 * Read startup arguments
	 */
	var argConfigFile = flag.String("config", "./config.toml", "Path to configuration file \"config.toml\".")
	flag.Parse()

	/*
	 * Read config file
	 */
	err := readConfig(*argConfigFile, &conf)
	if err != nil {
		log.Println("There was an error while reading the configuration file:", err)
	}

	/*
	 * Start HTTP server
	 */
	log.Println("Starting up XMPP HTTP upload server ...")
	http.HandleFunc("/"+conf.UploadSubDir, handleRequest)
	log.Printf("Server started on port %s. Waiting for requests.\n", conf.Listenport)
	http.ListenAndServe(conf.Listenport, nil)
}
