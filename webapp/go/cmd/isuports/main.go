package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	isuports "github.com/isucon/isucon12-qualify/webapp/go"
)

func main() {
	go func() {
		if err := http.ListenAndServe("127.0.0.1:6060", nil); err != nil {
			log.Printf("pprof server: %v", err)
		}
	}()
	isuports.Run()
}
