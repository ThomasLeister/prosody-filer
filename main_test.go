package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadValid(t *testing.T) {
	// Set config
	readConfig("config.toml")

	// Read catmetal file
	catmetalfile, err := ioutil.ReadFile("catmetal.jpg")
	if err != nil {
		t.Fatal(err)
	}

	// Create request
	req, err := http.NewRequest("PUT", "/upload/thomas/abc/catmetal.jpg", bytes.NewBuffer(catmetalfile))
	q := req.URL.Query()
	q.Add("v", "e17531b1e88bc9a5cbf816eca8a82fc09969c9245250f3e1b2e473bb564e4be0")
	req.URL.RawQuery = q.Encode()

	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleRequest)

	// Send request and record response
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v. HTTP body: %s", status, http.StatusOK, rr.Body.String())
	}
}

func TestUploadMissingMAC(t *testing.T) {
	// Set config
	readConfig("config.toml")

	// Read catmetal file
	catmetalfile, err := ioutil.ReadFile("catmetal.jpg")
	if err != nil {
		t.Fatal(err)
	}

	// Create request
	req, err := http.NewRequest("PUT", "/upload/thomas/abc/catmetal.jpg", bytes.NewBuffer(catmetalfile))

	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleRequest)

	// Send request and record response
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusConflict {
		t.Errorf("handler returned wrong status code: got %v want %v. HTTP body: %s", status, http.StatusConflict, rr.Body.String())
	}
}

func TestUploadInvalidMAC(t *testing.T) {
	// Set config
	readConfig("config.toml")

	// Read catmetal file
	catmetalfile, err := ioutil.ReadFile("catmetal.jpg")
	if err != nil {
		t.Fatal(err)
	}

	// Create request
	req, err := http.NewRequest("PUT", "/upload/thomas/abc/catmetal.jpg", bytes.NewBuffer(catmetalfile))
	q := req.URL.Query()
	q.Add("v", "abc")
	req.URL.RawQuery = q.Encode()

	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleRequest)

	// Send request and record response
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusForbidden {
		t.Errorf("handler returned wrong status code: got %v want %v. HTTP body: %s", status, http.StatusForbidden, rr.Body.String())
	}
}

func TestUploadInvalidMethod(t *testing.T) {
	// Set config
	readConfig("config.toml")

	// Read catmetal file
	catmetalfile, err := ioutil.ReadFile("catmetal.jpg")
	if err != nil {
		t.Fatal(err)
	}

	// Create request
	req, err := http.NewRequest("POST", "/upload/thomas/abc/catmetal.jpg", bytes.NewBuffer(catmetalfile))

	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleRequest)

	// Send request and record response
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v. HTTP body: %s", status, http.StatusMethodNotAllowed, rr.Body.String())
	}
}
