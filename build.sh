#!/bin/sh

##
## Builds static prosody-filer binary
##

CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' .
