package main

import "fmt"

// Version is set at build time.
var Version = "dev"

func main() {
	fmt.Printf("go-app %s\n", Version)
}

// Add returns the sum of two integers.
func Add(a, b int) int {
	return a + b
}

// Multiply returns the product of two integers.
func Multiply(a, b int) int {
	return a * b
}
