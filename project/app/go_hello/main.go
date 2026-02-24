package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	hostname, _ := os.Hostname()
	fmt.Printf("Hello, World!\n")
	fmt.Printf("Running on: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Hostname: %s\n", hostname)
}
