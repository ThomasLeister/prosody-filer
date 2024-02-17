package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func createFile(absFilename string, fileStorePath string, w http.ResponseWriter, r *http.Request) error {
	// Make sure the directory path exists
	absDirectory := filepath.Dir(absFilename)
	err := os.MkdirAll(absDirectory, os.ModePerm)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return fmt.Errorf("failed to create directory %s: %s", absDirectory, err)
	}

	// Make sure the target file exists (MUST NOT exist before! -> O_EXCL)
	targetFile, err := os.OpenFile(absFilename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Conflict", http.StatusConflict)
		return fmt.Errorf("failed to create file %s: %s", absFilename, err)
	}
	defer targetFile.Close()

	// Copy file contents to file
	_, err = io.Copy(targetFile, r.Body)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return fmt.Errorf("failed to copy file contents to %s: %s", absFilename, err)
	}

	w.WriteHeader(http.StatusCreated)
	return nil
}
