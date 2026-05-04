package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/ready", readyHandler)
	http.HandleFunc("/fraud-score", fraudScoreHandler)

	if sockPath := os.Getenv("SOCKET_PATH"); sockPath != "" {
		// Stale socket from a previous run would make Listen fail with EADDRINUSE.
		_ = os.Remove(sockPath)
		ln, err := net.Listen("unix", sockPath)
		if err != nil {
			panic(err)
		}
		// 0666 so nginx (running as a different user) can connect.
		if err := os.Chmod(sockPath, 0666); err != nil {
			panic(err)
		}
		fmt.Println("Server starting on unix:", sockPath)
		if err := http.Serve(ln, nil); err != nil {
			panic(err)
		}
		return
	}

	fmt.Println("Server starting on :9999")
	if err := http.ListenAndServe(":9999", nil); err != nil {
		panic(err)
	}
}
