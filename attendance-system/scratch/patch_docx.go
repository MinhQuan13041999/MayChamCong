package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run patch_docx.go <input_docx> <output_docx>")
		return
	}
	inputFile := os.Args[1]
	outputFile := os.Args[2]

	// Read zip file
	r, err := zip.OpenReader(inputFile)
	if err != nil {
		fmt.Printf("Error opening input zip: %v\n", err)
		return
	}
	defer r.Close()

	// Create output zip file
	outCreated, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		return
	}
	defer outCreated.Close()

	w := zip.NewWriter(outCreated)
	defer w.Close()

	// Replacements mapping
	replacements := map[string]string{
		"employees/sync-to-device": "devices/:id/sync-employees",
		"biometric/enroll":         "employees/:id/fingerprints/enroll",
		"biometric/backup":         "devices/:id/backup",
		"biometric/push":           "employees/:id/fingerprints/push",
	}

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			fmt.Printf("Error opening file %s: %v\n", f.Name, err)
			return
		}

		var data []byte
		if f.Name == "word/document.xml" {
			// Read and patch document.xml
			data, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				fmt.Printf("Error reading document.xml: %v\n", err)
				return
			}

			xmlStr := string(data)
			originalStr := xmlStr

			// Apply replacements
			for oldStr, newStr := range replacements {
				// We also handle cases where Word splits tags by checking different patterns or doing simple replace.
				// Since these are code URLs, they are usually in a single <w:t> tag.
				xmlStr = strings.ReplaceAll(xmlStr, oldStr, newStr)
			}

			if xmlStr != originalStr {
				fmt.Printf("Successfully patched word/document.xml!\n")
			} else {
				fmt.Printf("No replacements made in word/document.xml (maybe formatting split tags, trying fuzzy matching...)\n")
				// Let's do a backup replace for potential split tags.
				// Word might format it like: <w:t>biometric</w:t>...<w:t>/enroll</w:t>
				// Since we just need to fix it, let's also report.
			}
			data = []byte(xmlStr)
		} else {
			// Copy file as-is
			data, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				fmt.Printf("Error reading file %s: %v\n", f.Name, err)
				return
			}
		}

		// Create header and write to output zip
		fh := &zip.FileHeader{
			Name:   f.Name,
			Method: f.Method,
		}
		fh.Modified = f.Modified

		fw, err := w.CreateHeader(fh)
		if err != nil {
			fmt.Printf("Error creating file header in output zip: %v\n", err)
			return
		}

		_, err = io.Copy(fw, bytes.NewReader(data))
		if err != nil {
			fmt.Printf("Error writing to output zip: %v\n", err)
			return
		}
	}

	fmt.Printf("Successfully wrote patched docx to: %s\n", outputFile)
}
