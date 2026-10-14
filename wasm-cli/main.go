package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	method := os.Getenv("REQUEST_METHOD")
	path := os.Getenv("PATH_INFO")

	if method != "GET" {
		fmt.Println("Status: 405 Method Not Allowed")
		fmt.Println("Content-Type: text/plain")
		fmt.Println()
		fmt.Println("Method not allowed")
		return
	}

	if path != "/time" {
		fmt.Println("Status: 404 Not Found")
		fmt.Println("Content-Type: text/plain")
		fmt.Println()
		fmt.Println("Not found")
		return
	}

	moscow := time.FixedZone("MSK", 3*60*60)
	now := time.Now().In(moscow)

	fmt.Println("Content-Type: application/json")
	fmt.Println()

	fmt.Printf(
		"{\"unix\":%d,\"iso\":%q,\"hour_minute\":%q}\n",
		now.Unix(),
		now.Format(time.RFC3339),
		now.Format("15:04"),
	)
}
