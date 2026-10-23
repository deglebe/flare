# flare

a small, [fasthttp](https://github.com/valyala/fasthttp)-backed router with a
gin-like handler, batteries not included.

**module:** [`github.com/deglebe/flare`](https://github.com/deglebe/flare)

```go
package main

import (
    "log"

    "github.com/deglebe/flare"
)

func main() {
    e := flare.New()
    e.Use(flare.Recover())

    e.GET("/health", func(c *flare.Ctx) {
        c.SetBodyString("ok")
    })
    e.GET("/users/:id", func(c *flare.Ctx) {
        c.JSON(200, map[string]string{"id": c.Param("id")})
    })

    log.Fatal(e.Listen(":8080"))
}
```

## is it any good?

[yes](https://news.ycombinator.com/item?id=3067434)

## install

```sh
go get github.com/deglebe/flare
```

requires go 1.25.5+ (see go.mod). for faster json
encoding/decoding using bytedance's sonic (marginal boost on small apis):

```bash
go build -tags flare_sonic
```

## examples

from the repo root:

```bash
go run ./examples/basic    # / /health /hello/:name
go run ./examples/api      # token-gated json crud
go run ./examples/static   # get / -> public/index.html
```

```bash
# basic
curl -s localhost:8080/hello/ada

# api (header required)
curl -s -H 'X-Token: secret' localhost:8080/api/users
curl -s -H 'X-Token: secret' -H 'Content-Type: application/json' \
  -d '{"name":"linus"}' localhost:8080/api/users

# static
curl -s localhost:8080/
curl -s localhost:8080/health
```

## how it works

### routing

each http method owns a radix tree (`:param` and `*catchall`). trailing slashes
are **strict**: `/a` and `/a/` are different routes. flare never redirects or
strips them.

registration freezes on the first `Handler()`/`Listen()`. at freeze, flare:

1. collapses an empty method root that has a single child (reduces hops per
request by one)
2. builds an exact-path table for static leaves so common paths skip walking the
full tree.

the seven usual verbs use a fixed method index instead of a map. unknown methods
panic at registration.

### middleware and `Ctx`

`Use` and `Group` precompose middleware into each route's handler slice when the
route is registered, meaning that there is no per-request stack rebuild. `Next`,
`Abort`/`AbortWithStatus`/`AbortWithStatusJSON`, and `Recover`/`RecoverWith`
follow the usual pattern.

each request borrows a pooled `Ctx` wrapping `*fasthttp.RequestCtx`. routes with
a single leaf handler call it directly and skip the `Next` index loop.

### requests and responses

helpers stay optional and off the match path:

| | |
|--|--|
| params/query/headers | `Param`, `ParamInt`, `Query`, `GetHeader` |
| json | `JSON`, `BindJSON` (`-tags flare_sonic` optional) |
| forms | `Bind`, `BindForm` |
| content negotiate | `Negotiate` (html vs json stub) |
| files | `Static(prefix, dir)`, `StaticFS(prefix, fs)` |
| client address | `ClientIP` (see below) |

`Engine.Server` is an embedded `fasthttp.Server` with conservative defaults
(4 mib body, 10 s read/write, 60 s idle). override fields before `Listen`.

`NoRoute` / `NoMethod` distinguish unknown paths (404) from wrong methods on a
known path (405).

### client ip and proxies

by default `ClientIP()` returns the socket peer and **ignores**
`X-Forwarded-For`/`X-Real-IP`. behind nginx or similar, you must allow the proxy
hop explicitly:

```go
_ = e.SetTrustedProxies("10.0.0.0/8", "127.0.0.1")
// optional: e.SetRemoteIPHeaders("X-Forwarded-For")
```

trusted peers may supply those headers; `X-Forwarded-For` is walked
right-to-left, skipping hops that are also trusted.

### scope

flare is meant to be a minimal router kernel on top of which to build an
equally, if not more performant stack. so, flare is only match, groups,
middleware, `Ctx`, bind/json helpers, static files, and listen/limits, until
more is called for in core. sessions, oauth, openapi, orms, heavy validators,
and cors/rate-limit packs are intentionally left out, though a companion library
may include these features in the future.

