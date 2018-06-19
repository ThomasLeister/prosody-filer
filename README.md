# Prosody Filer

A simple file server for handling http_upload clients requests. This server can be used with the [mod_http_upload_external Module](https://modules.prosody.im/mod_http_upload_external.html) of Prosody. 

*Why should I use this server?*

* Prosody's integrated file server is said to be memory leaking.
* This server works without any script interpreters or additional dependencies. It is delivered as a binary.
* Go is very good at serving HTTP requests.

*Why shoud I NOT use this server?*

* It has not been tested, yet and is still under development.


# Compilation
To compile the server, you need a full Golang development environment. This can be set up quickly: https://golang.org/doc/install#install

Then checkout this repo: 

    go get github.com/ThomasLeister/prosody-filer

and switch to the new directory: 

    cd $GOPATH/src/github.com/ThomasLeister/prosody-filer

The source code can now be build: 

    go build main.go


# Installation

[TBD]


