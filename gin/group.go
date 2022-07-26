package gin

// RouterGroup 路由分组的结构体
type RouterGroup struct {
	parent     *RouterGroup
	engine     *Engine
	basePath   string
	middleware []HandlerFunc
}

func (e *Engine) NewGroup(basePath string) *RouterGroup {
	return &RouterGroup{
		parent:   nil,
		engine:   e,
		basePath: basePath,
	}
}

func (g *RouterGroup) NewGroup(basePath string) *RouterGroup {
	return &RouterGroup{
		parent:   g,
		engine:   g.engine,
		basePath: basePath,
	}
}

func (g *RouterGroup) AddRouter(method string, path string, handler HandlerFunc) {
	g.engine.AddRouter(method, g.basePath+path, handler)
}

func (g *RouterGroup) GET(path string, handler HandlerFunc) {
	g.engine.GET(g.basePath+path, handler)
}

func (g *RouterGroup) POST(path string, handler HandlerFunc) {
	g.engine.POST(g.basePath+path, handler)
}

func (g *RouterGroup) PUT(path string, handler HandlerFunc) {
	g.engine.PUT(g.basePath+path, handler)
}

func (g *RouterGroup) DELETE(path string, handler HandlerFunc) {
	g.engine.DELETE(g.basePath+path, handler)
}
