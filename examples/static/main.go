package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/deglebe/flare"
)

func main() {
	dir := filepath.Join(".", "public")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	index := filepath.Join(dir, "index.html")
	if _, err := os.Stat(index); err != nil {
		if err := os.WriteFile(index, []byte("<!doctype html><h1>flare static</h1>\n"), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	e := flare.New()
	e.Use(flare.Recover())
	e.GET("/health", func(c *flare.Ctx) { c.SetBodyString("ok") })
	e.Static("/", dir)

	log.Printf("open http://127.0.0.1:8080/  (files from %s)", dir)
	log.Fatal(e.Listen(":8080"))
}
