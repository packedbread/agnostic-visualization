package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8008"
	}

	ConfigureCache()
	defer Memcached.Quit()

	ConfigureRegistry()

	router := SetupHandles()
	http.Handle("/", router)
	log.Fatal(http.ListenAndServe(":" + port, nil))
}
