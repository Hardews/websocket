package gin

import (
	"fmt"
	"log"
	"net/http"
)

type Engine struct {
	RouterGroup
	middleware []HandlerFunc
	trees      tree
}

type HandlerFunc func(ctx *Context)

func New() *Engine {
	engine := &Engine{
		trees: newTree(),
	}
	return engine
}

func Default() *Engine {
	return New()
}

func (e *Engine) Run(addr ...string) error {
	var address = ":8080"
	if addr != nil {
		address = addr[0]
	}
	return http.ListenAndServe(address, e)
}

func (e *Engine) GET(path string, handler HandlerFunc) {
	e.addRouter(http.MethodGet, path, handler)
}

func (e *Engine) POST(path string, handler HandlerFunc) {
	e.addRouter(http.MethodPost, path, handler)
}

func (e *Engine) PUT(path string, handler HandlerFunc) {
	e.addRouter(http.MethodPut, path, handler)
}

func (e *Engine) DELETE(path string, handler HandlerFunc) {
	e.addRouter(http.MethodDelete, path, handler)
}

func (e *Engine) HandleFunc(method string, path string, handler HandlerFunc) {
	e.addRouter(method, path, handler)
}

func (e *Engine) addRouter(method string, path string, handler HandlerFunc) {
	if path[0] != '/' {
		log.Println("path must begin with '/'")
		return
	}
	if handler == nil {
		log.Println("HTTP method can not be empty")
		return
	}
	if method == "" {
		log.Println("there must be at least one handler")
		return
	}

	root, ok := e.trees[method]
	if !ok {
		root = new(node)
		e.trees[method] = root
	}

	root.add(path, handler)
}

func (e *Engine) Use(middleware HandlerFunc) {
	e.middleware = append(e.middleware, middleware)
}

func (g *RouterGroup) Use(middleware HandlerFunc) {
	g.engine.middleware = append(g.engine.middleware, middleware)
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	key := req.Method
	ctx := newContext(w, req)

	if e.middleware != nil {
		for _, handlerFunc := range e.middleware {
			handlerFunc(ctx)
		}
	}

	if e.RouterGroup.middleware != nil {
		for _, handlerFunc := range e.RouterGroup.middleware {
			handlerFunc(ctx)
		}
	}

	if n, ok := e.trees[key]; ok {
		handler := n.getValue(ctx.Request.URL.Path)
		if handler == nil {
			fmt.Fprintf(w, "404 NOT FOUND: %s\n", req.URL)
		} else {
			handler(ctx)
		}
	} else {
		fmt.Fprintf(w, "404 NOT FOUND: %s\n", req.URL)
	}
}
