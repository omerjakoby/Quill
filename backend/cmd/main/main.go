package main

import (
	"fmt"
	"quill/pkg/transport/quill"
)

func main() {
	fmt.Println("Starting Quill")
	quill.OpenTcpPort()

}
