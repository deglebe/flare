package main

import (
	"log"
	"sync"

	"github.com/deglebe/flare"
	"github.com/valyala/fasthttp"
)

type user struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var (
	mu    sync.Mutex
	users = map[int]user{1: {ID: 1, Name: "ada"}, 2: {ID: 2, Name: "grace"}}
	next  = 3
)

func main() {
	e := flare.New()
	e.Use(flare.Recover())
	_ = e.SetTrustedProxies("127.0.0.1", "::1")

	e.GET("/health", func(c *flare.Ctx) { c.SetBodyString("ok") })

	api := e.Group("/api", requireToken)
	api.GET("/users", listUsers)
	api.GET("/users/:id", getUser)
	api.POST("/users", createUser)
	api.DELETE("/users/:id", deleteUser)

	log.Println("API token header: X-Token: secret")
	log.Fatal(e.Listen(":8080"))
}

func requireToken(c *flare.Ctx) {
	if c.GetHeader("X-Token") != "secret" {
		c.AbortWithStatusJSON(fasthttp.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	c.Next()
}

func listUsers(c *flare.Ctx) {
	mu.Lock()
	out := make([]user, 0, len(users))
	for _, u := range users {
		out = append(out, u)
	}
	mu.Unlock()
	c.JSON(fasthttp.StatusOK, out)
}

func getUser(c *flare.Ctx) {
	id, err := c.ParamInt("id")
	if err != nil {
		c.AbortWithStatusJSON(fasthttp.StatusBadRequest, map[string]string{"error": "bad id"})
		return
	}
	mu.Lock()
	u, ok := users[id]
	mu.Unlock()
	if !ok {
		c.AbortWithStatusJSON(fasthttp.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	c.JSON(fasthttp.StatusOK, u)
}

func createUser(c *flare.Ctx) {
	var in user
	if err := c.BindJSON(&in); err != nil || in.Name == "" {
		c.AbortWithStatusJSON(fasthttp.StatusBadRequest, map[string]string{"error": "bad body"})
		return
	}
	mu.Lock()
	in.ID = next
	next++
	users[in.ID] = in
	mu.Unlock()
	c.JSON(fasthttp.StatusCreated, in)
}

func deleteUser(c *flare.Ctx) {
	id, err := c.ParamInt("id")
	if err != nil {
		c.AbortWithStatus(fasthttp.StatusBadRequest)
		return
	}
	mu.Lock()
	_, ok := users[id]
	delete(users, id)
	mu.Unlock()
	if !ok {
		c.AbortWithStatusJSON(fasthttp.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	c.SetStatusCode(fasthttp.StatusNoContent)
}
