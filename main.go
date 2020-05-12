package main

import (
	"log"
	"net/http"
)

func main() {
	ConfigureCache()
	defer Memcached.Quit()

	ConfigureRegistry()

	router := SetupHandles()
	http.Handle("/", router)
	log.Fatal(http.ListenAndServe("127.0.0.1:8008", nil))
}
