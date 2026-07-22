package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("TAI_LIEU_BAN_GIAO_KY_THUAT_V3.docx.txt")
	if err != nil {
		panic(err)
	}

	content := string(data)
	target := "4.1. Danh sách chi tiết Swagger API Endpoints"
	idx := strings.Index(content, target)
	if idx == -1 {
		fmt.Println("Target section not found")
		return
	}

	sub := content[idx:]
	if len(sub) > 2500 {
		sub = sub[:2500]
	}
	fmt.Println(sub)
}
