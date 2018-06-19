/*
 * This module allows upload via mod_http_upload_external
 * Send with:
 * curl -X PUT "http://localhost:5050/upload/thomas/abc/catmetal.jpg?v=e17531b1e88bc9a5cbf816eca8a82fc09969c9245250f3e1b2e473bb564e4be0" --data-binary '@catmetal.jpg'
 * Secret: 123
 * HMAC: e17531b1e88bc9a5cbf816eca8a82fc09969c9245250f3e1b2e473bb564e4be0
 */

/*
 * TODO:
 * - Make use of HMAC Equals()
 * - Make sure that software does not crash
 * - Implement unit test
 * - Implement auto deletion?
 */

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"log"
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
	Listenport    int64
	Secret        string
	UploadRootDir string
}

var conf Config

/*
 * Upload handler
 * Is activated when a clients requests the file, file information or an upload
 */
func upload(w http.ResponseWriter, r *http.Request) {
	log.Println("Method:", r.Method)

	// Parse URL and args
	u, _ := url.Parse(r.RequestURI)
	a, _ := url.ParseQuery(u.RawQuery)

	fileStorePath := strings.TrimLeft(u.Path, "upload/")

	if r.Method == "PUT" {
		/*
		 * Check if the request is valid
		 */
		mac := hmac.New(sha256.New, []byte(conf.Secret))
		//mac.Write([]byte("thomas/abc/catmetal.jpg 23026"))
		mac.Write([]byte(fileStorePath + " " + strconv.FormatInt(r.ContentLength, 10)))
		macString := hex.EncodeToString(mac.Sum(nil))

		log.Println("Expected MAC:", macString)
		log.Println("Got MAC:", a["v"][0])

		if macString == a["v"][0] {
			log.Println("MAC is correct!")

			log.Println("Storing file in", fileStorePath)

			// Make sure the path exists
			os.MkdirAll(filepath.Dir(conf.UploadRootDir+fileStorePath), os.ModePerm)

			file, err := os.Create(conf.UploadRootDir + fileStorePath)
			if err != nil {
				log.Println("Creating new file failed:", err)
				http.Error(w, "409 Conflict", 409)
			}

			n, err := io.Copy(file, r.Body)
			if err != nil {
				log.Println("Writing to new file failed:", err)
				http.Error(w, "409 Conflict", 409)
			}

			log.Println(n, "Bytes written")
		} else {
			log.Println("Invalid MAC.")
			http.Error(w, "403 Forbidden", 403)
		}
	} else if r.Method == "HEAD" {
		fileinfo, err := os.Stat(conf.UploadRootDir + fileStorePath)
		if err != nil {
			log.Println("Getting file information failed:", err)
			http.Error(w, "404 Not Found", 404)
			return
		}
		w.Header().Set("Content-Length", strconv.FormatInt(fileinfo.Size(), 10))
	} else if r.Method == "GET" {
		http.ServeFile(w, r, conf.UploadRootDir+fileStorePath)
	} else {
		log.Println("Invalid method", r.Method, "for access to /upload.")
		http.Error(w, "405 Method Not Allowed", 405)
	}
}

/*
 * Main function
 */
func main() {
	log.Println("Starting up XMPP HTTP upload server ...")
	log.Println("Reading configuration ...")

	configdata, err := ioutil.ReadFile("./config.toml")
	if err != nil {
		log.Fatalln("Configuration file config.toml cannot be read:", err, "...Exiting.")
	}

	if _, err := toml.Decode(string(configdata), &conf); err != nil {
		log.Fatalln("Config file config.toml is invalid:", err)
	}

	http.HandleFunc("/upload/", upload)
	log.Printf("Server started on port %d. Waiting for requests.", conf.Listenport)
	http.ListenAndServe(":"+strconv.FormatInt(conf.Listenport, 10), nil)
}
