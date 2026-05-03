package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/ready", readyHandler)
	http.HandleFunc("/fraud-score", fraudScoreHandler)

	fmt.Println("Server starting on :9999")
	if err := http.ListenAndServe(":9999", nil); err != nil {
		panic(err)
	}
}
