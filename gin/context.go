package gin

import (
	"encoding/json"
	"log"
	"net/http"
)

type Context struct {
	Request    *http.Request
	Writer     http.ResponseWriter
	StatusCode int
}

type H map[string]interface{}

func newContext(w http.ResponseWriter, req *http.Request) *Context {
	return &Context{
		Request: req,
		Writer:  w,
	}
}

func (c *Context) PostForm(key string) string {
	return c.Request.FormValue(key)
}

func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

func (c *Context) Status(code int) {
	c.StatusCode = code
	c.Writer.WriteHeader(code)
}

func (c *Context) SetHeader(key string, value string) {
	c.Writer.Header().Set(key, value)
}

func (c *Context) JSON(code int, obj interface{}) {
	c.SetHeader("Content-Type", "application/json")
	c.Status(code)
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(obj); err != nil {
		http.Error(c.Writer, err.Error(), 500)
		log.Println(500, "  ", c.Request.Method, "  ", c.Request.URL)
	}
	log.Println(code, "  ", c.Request.Method, "  ", c.Request.URL)
}
