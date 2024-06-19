// Package test provides tools for testing the library
package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/invopop/gobl"
)

// LoadTestEnvelope returns a GOBL Envelope from a file in the `test/data` folder
func LoadTestEnvelope(name string) (*gobl.Envelope, error) {
	data, _ := os.ReadFile(name)

	env := new(gobl.Envelope)
	if err := json.Unmarshal(data, env); err != nil {
		return nil, err
	}

	return env, nil

}

// GetDataGlob returns a list of files in the `test/data` folder that match the pattern
func GetDataGlob(pattern string) ([]string, error) {
	return filepath.Glob(filepath.Join(GetDataPath(), pattern))
}

// GetDataPath returns the path to the `test/data` folder
func GetDataPath() string {
	return filepath.Join(GetTestPath(), "data")
}

// GetTestPath returns the path to the `test` folder
func GetTestPath() string {
	return filepath.Join(getRootFolder(), "test")
}

func getRootFolder() string {
	cwd, _ := os.Getwd()

	for !isRootFolder(cwd) {
		cwd = removeLastEntry(cwd)
	}

	return cwd
}

func isRootFolder(dir string) bool {
	files, _ := os.ReadDir(dir)

	for _, file := range files {
		if file.Name() == "go.mod" {
			return true
		}
	}

	return false
}

func removeLastEntry(dir string) string {
	lastEntry := "/" + filepath.Base(dir)
	i := strings.LastIndex(dir, lastEntry)
	return dir[:i]
}
