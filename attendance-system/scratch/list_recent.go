package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)

	// Only walk specific directories to avoid scanning huge backups/node_modules/gocache
	dirsToWalk := []string{"internal", "web", "cmd", "migrations"}

	for _, dir := range dirsToWalk {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if info.ModTime().After(cutoff) {
				fmt.Printf("%s - %s\n", path, info.ModTime().Format("2006-01-02 15:04:05"))
			}
			return nil
		})
		if err != nil {
			fmt.Printf("Error walking %s: %v\n", dir, err)
		}
	}

	// Also check files in the root directory itself
	files, err := os.ReadDir(".")
	if err == nil {
		for _, f := range files {
			if !f.IsDir() {
				info, err := f.Info()
				if err == nil && info.ModTime().After(cutoff) {
					fmt.Printf("%s - %s\n", info.Name(), info.ModTime().Format("2006-01-02 15:04:05"))
				}
			}
		}
	}
}
