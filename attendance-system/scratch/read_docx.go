package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run read_docx.go <path_to_docx>")
		return
	}
	path := os.Args[1]

	r, err := zip.OpenReader(path)
	if err != nil {
		fmt.Printf("Error opening zip: %v\n", err)
		return
	}
	defer r.Close()

	var docXML io.ReadCloser
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docXML, err = f.Open()
			if err != nil {
				fmt.Printf("Error opening document.xml: %v\n", err)
				return
			}
			break
		}
	}

	if docXML == nil {
		fmt.Println("word/document.xml not found in zip archive")
		return
	}
	defer docXML.Close()

	content, err := io.ReadAll(docXML)
	if err != nil {
		fmt.Printf("Error reading document.xml: %v\n", err)
		return
	}

	// Simple regex to extract text inside XML tags <w:t>...</w:t>
	re := regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)
	matches := re.FindAllStringSubmatch(string(content), -1)

	var sb strings.Builder
	for _, match := range matches {
		if len(match) > 1 {
			sb.WriteString(match[1])
		}
	}

	outputText := sb.String()
	// Replace some common spacing issues
	outputText = strings.ReplaceAll(outputText, "</w:p>", "\n")
	outputText = strings.ReplaceAll(outputText, "<w:br/>", "\n")

	// Print first 5000 characters
	fmt.Println("--- FIRST 5000 CHARACTERS OF THE DOCX ---")
	if len(outputText) > 5000 {
		fmt.Println(outputText[:5000])
		fmt.Println("\n... (TRUNCATED) ...")
	} else {
		fmt.Println(outputText)
	}

	// Write to temporary text file for full inspection if needed
	txtPath := path + ".txt"
	err = os.WriteFile(txtPath, []byte(outputText), 0644)
	if err == nil {
		fmt.Printf("\nSaved full extracted text to: %s\n", txtPath)
	}
}
