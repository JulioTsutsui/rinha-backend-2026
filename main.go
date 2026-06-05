package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"runtime/debug"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "build-index" {
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: server build-index <refs.json[.gz]> <out.bin>")
			os.Exit(1)
		}
		if err := buildIndex(os.Args[2], os.Args[3]); err != nil {
			panic(err)
		}
		return
	}

	runtime.GOMAXPROCS(1)
	debug.SetGCPercent(200)
	debug.SetMemoryLimit(140 << 20)

	indexPath := os.Getenv("INDEX_PATH")
	if indexPath == "" {
		indexPath = "index.bin"
	}
	ix, err := openIndex(indexPath)
	if err != nil {
		panic(err)
	}
	fmt.Printf("index loaded: N=%d K=%d blocks=%d\n", ix.N, ix.K, ix.NBlocks)

	k := ix.K
	scratchPool.New = func() any { return newScratch(k) }

	var ln net.Listener
	if sock := os.Getenv("SOCKET_PATH"); sock != "" {
		_ = os.Remove(sock)
		ln, err = net.Listen("unix", sock)
		if err != nil {
			panic(err)
		}
		_ = os.Chmod(sock, 0666)
		fmt.Println("listening on unix:", sock)
	} else {
		ln, err = net.Listen("tcp", ":9999")
		if err != nil {
			panic(err)
		}
		fmt.Println("listening on :9999")
	}
	serve(ln, ix)
}
