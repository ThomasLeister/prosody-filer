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
	"github.com/sirupsen/logrus"
)

/*
 * Configuration of this server
 */
type Config struct {
	ListenPort   string
	UnixSocket   bool
	Secret       string
	StoreDir     string
	UploadSubDir string
	LogLevel     string
}

var conf Config
var versionString string = "0.0.0"

var log = &logrus.Logger{
	Out:       os.Stdout,
	Formatter: new(logrus.TextFormatter),
	Hooks:     make(logrus.LevelHooks),
	Level:     logrus.DebugLevel,
}

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
	log.Info("Incoming request: ", r.Method, r.URL.String())

	// Parse URL and args
	p := r.URL.Path

	a, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		log.Warn("Failed to parse query")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	subDir := path.Join("/", conf.UploadSubDir)
	fileStorePath := strings.TrimPrefix(p, subDir)
	if fileStorePath == "" || fileStorePath == "/" {
		log.Warn("Access to / forbidden")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	} else if fileStorePath[0] == '/' {
		fileStorePath = fileStorePath[1:]
	}

	absFilename := filepath.Join(conf.StoreDir, fileStorePath)

	// Add CORS headers
	addCORSheaders(w)

	if r.Method == http.MethodPut {
		/*
		 * User client tries to upload file
		 */

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
			log.Warn("No HMAC attached to URL. Expecting URL with \"v\", \"v2\" or \"token\" parameter as MAC")
			http.Error(w, "No HMAC attached to URL. Expecting URL with \"v\", \"v2\" or \"token\" parameter as MAC", http.StatusForbidden)
			return
		}

		// Init HMAC
		mac := hmac.New(sha256.New, []byte(conf.Secret))
		macString := ""

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

		/*
		 * Check whether calculated (expected) MAC is the MAC that client send in "v" URL parameter
		 */
		if hmac.Equal([]byte(macString), []byte(a[protocolVersion][0])) {
			err = createFile(absFilename, fileStorePath, w, r)
			if err != nil {
				log.Error(err)
			}
			return
		} else {
			log.Warning("Invalid MAC.")
			http.Error(w, "Invalid MAC", http.StatusForbidden)
			return
		}
	} else if r.Method == http.MethodHead || r.Method == http.MethodGet {
		/*
		 * User client tries to download a file
		 */

		fileInfo, err := os.Stat(absFilename)
		if err != nil {
			log.Error("Getting file information failed:", err)
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		} else if fileInfo.IsDir() {
			log.Warning("Directory listing forbidden!")
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
			w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
		} else {
			http.ServeFile(w, r, absFilename)
		}

		return
	} else if r.Method == http.MethodOptions {
		// Client CORS request: Return allowed methods
		w.Header().Set("Allow", ALLOWED_METHODS)
		return
	} else {
		// Client is using a prohibited / unsupported method
		log.Warn("Invalid method", r.Method, "for access to ", conf.UploadSubDir)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func readConfig(configFilename string, conf *Config) error {
	configData, err := os.ReadFile(configFilename)
	if err != nil {
		log.Fatal("Configuration file config.toml cannot be read:", err, "...Exiting.")
		return err
	}

	if _, err := toml.Decode(string(configData), conf); err != nil {
		log.Fatal("Config file config.toml is invalid:", err)
		return err
	}

	return nil
}

func setLogLevel() {
	switch conf.LogLevel {
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.WarnLevel)
		fmt.Print("Invalid log level set in config. Defaulting to \"warn\"")
	}
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

	// Select proto
	if conf.UnixSocket {
		proto = "unix"
	} else {
		proto = "tcp"
	}

	/*
	 * Start HTTP server
	 */
	log.Println("Starting prosody-filer", versionString, "...")
	listener, err := net.Listen(proto, conf.ListenPort)
	if err != nil {
		log.Fatalln("Could not open listening socket:", err)
	}

	subpath := path.Join("/", conf.UploadSubDir)
	subpath = strings.TrimRight(subpath, "/")
	subpath += "/"
	http.HandleFunc(subpath, handleRequest)
	log.Printf("Server started on port %s. Waiting for requests.\n", conf.ListenPort)

	// Set log level
	setLogLevel()

	http.Serve(listener, nil)
	// This line will only be reached when quitting
}
