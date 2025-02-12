package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	portString := os.Getenv("PORT")
	if portString == "" {
		portString = "8080"
	}
	http.HandleFunc("/", handler)
	

	server := &http.Server{
		Addr: ":"+ portString,
		Handler: nil,
	}

	log.Printf("Serving to port: %s", portString)
	server.ListenAndServe()
}
