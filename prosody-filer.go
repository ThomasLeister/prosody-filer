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
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}

	subDir := path.Join("/", conf.UploadSubDir)
	fileStorePath := strings.TrimPrefix(p, subDir)
	if fileStorePath == "" || fileStorePath == "/" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	} else if fileStorePath[0] == '/' {
		fileStorePath = fileStorePath[1:]
	}

	absFilename := filepath.Join(conf.Storedir, fileStorePath)

	// Add CORS headers
	addCORSheaders(w)

	if r.Method == http.MethodPut {
		/*
			Check if MAC is attached to URL and check protocol version.
			Ejabberd: 	supports "v" and probably "v2"		Doc: https://docs.ejabberd.im/archive/20_12/modules/#mod-http-upload
			Prosody: 	supports "v" and "v2"				Doc: https://modules.prosody.im/mod_http_upload_external.html
			Metronome: 	supports: "token" (meaning "v2")	Doc: https://archon.im/metronome-im/documentation/external-upload-protocol/)
		*/
		var protocolVersion string
		if a["v2"] != nil {
			protocolVersion = "v2"
		} else if a["token"] != nil {
			protocolVersion = "token"
		} else if a["v"] != nil {
			protocolVersion = "v"
		} else {
			http.Error(w, "No HMAC attached to URL. Expecting URL with \"v\", \"v2\" or \"token\" parameter as MAC", http.StatusForbidden)
			return
		}

		// Init HMAC
		mac := hmac.New(sha256.New, []byte(conf.Secret))
		macString := ""

		//log info + MAC key generation
		//log.Println("fileStorePath:", fileStorePath)
		//log.Println("ContentLength:", strconv.FormatInt(r.ContentLength, 10))
		//log.Println("fileType:", contentType)
		//log.Println("Protocol version used:", protocolVersion)

		// Calculate MAC, depending on protocolVersion
		if protocolVersion == "v" {
			// use a space character (0x20) between components of MAC
			mac.Write([]byte(fileStorePath + "\x20" + strconv.FormatInt(r.ContentLength, 10)))
			macString = hex.EncodeToString(mac.Sum(nil))
		} else if protocolVersion == "v2" || protocolVersion == "token" {
			// Get content type (for v2 / token)
			contentType := mime.TypeByExtension(filepath.Ext(fileStorePath))
			if contentType == "" {
				contentType = "application/octet-stream"
			}

			// use a null byte character (0x00) between components of MAC
			mac.Write([]byte(fileStorePath + "\x00" + strconv.FormatInt(r.ContentLength, 10) + "\x00" + contentType))
			macString = hex.EncodeToString(mac.Sum(nil))
		}

		//Debug logging
		//fmt.Println("MAC v1 calculated : ", mac_v1_String)
		//fmt.Println("MAC v2 calculated : ", mac_v2_String)

		/*
		 * Check whether calculated (expected) MAC is the MAC that client send in "v" URL parameter
		 */
		if hmac.Equal([]byte(macString), []byte(a[protocolVersion][0])) {
			err = createFile(absFilename, fileStorePath, w, r)
			if err != nil {
				fmt.Print(err)
			}
			return
		} else {
			//Debug - log byte comparision
			//log.Println([]byte(mac_v1_String))
			//log.Println([]byte(mac_v2_String))
			//log.Println([]byte(a[protocolVersion][0]))
			http.Error(w, "Invalid MAC", http.StatusForbidden)
			return
		}
	} else if r.Method == http.MethodHead || r.Method == http.MethodGet {
		fileinfo, err := os.Stat(absFilename)
		if err != nil {
			log.Println("Getting file information failed:", err)
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		} else if fileinfo.IsDir() {
			log.Println("Directory listing forbidden!")
			http.Error(w, "Forbidden", http.StatusForbidden)
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
