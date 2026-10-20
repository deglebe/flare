package main

import (
	"log"

	"github.com/deglebe/flare"
)

func main() {
	e := flare.New()
	e.Use(flare.Recover())

	e.GET("/", func(c *flare.Ctx) {
		c.SetBodyString("flare ok")
	})
	e.GET("/health", func(c *flare.Ctx) {
		c.SetBodyString("ok")
	})
	e.GET("/hello/:name", func(c *flare.Ctx) {
		c.JSON(200, map[string]string{"hello": c.Param("name")})
	})

	log.Fatal(e.Listen(":8080"))
}
