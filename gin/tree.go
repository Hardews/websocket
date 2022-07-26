package gin

import "log"

// 基数树
type tree map[string]*node

type node struct {
	path     string
	indices  string // 孩子节点的首字母
	children []*node
	handler  HandlerFunc // 当前节点的处理函数
}

func newTree() tree {
	return make(map[string]*node)
}

func (n *node) add(path string, handler HandlerFunc) {
	if len(n.path) > 0 || len(n.children) > 0 {
	walk:
		max := min(len(n.path), len(path))
		i := 0
		// 找到相同前缀的index值
		for i < max && n.path[i] == path[i] {
			i++
		}

		if i == len(n.path) && len(n.path) == len(path) {
			// 如果相同字符正好与n.path相等，然后它俩长度也一样时，为重复路径
			log.Fatalln("the same path use different handler func")
		}

		// 如果相同字符的值小于n.path的值 那么将n.path的后面部分变为子节点，相同部分变为父节点
		if i < len(n.path) {
			child := &node{
				path:     n.path[i:],
				indices:  n.indices,
				children: n.children,
				handler:  n.handler,
			}
			n.path = n.path[:i]
			n.indices = string(n.path[i])
			n.children = []*node{child}
			n.handler = nil
		}

		// 处理子节点的相同字符
		if i < len(path) {
			c := path[i]
			for index := 0; index < len(n.indices); index++ {
				if c == n.indices[index] {
					n = n.children[index]
					path = path[i:]
					goto walk
				}
			}

			//把新请求的path加入到router中
			n.insertChild(path[i:], path, handler, i)
			return
		}

	} else {
		n.path = path
		n.handler = handler
	}
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func (n *node) insertChild(path string, fullPath string, handler HandlerFunc, index int) {
	child := node{}
	child.handler = handler
	child.indices = ""
	child.path = path
	n.indices += string([]byte{fullPath[index]})
	n.children = append(n.children, &child)
}

func (n *node) getValue(path string) (handlers HandlerFunc) {
	index := 1
	max := min(len(n.path), len(path))
	i := 0
	// 找到相同前缀的index值
	for i < max && n.path[i] == path[i] {
		i++
	}
	// 如果没有相同的前缀值就是没有这个路径的方法
	if i == 0 {
		return nil
	}
walk:
	for {
		if len(path) > len(n.path) {
			path = path[i:]
			c := path[0]
			for i := 0; i < len(n.indices); i++ {
				if c == n.indices[i] {
					n = n.children[i]
					index++
					goto walk
				}
			}
			if index == 1 {
				return nil
			}
			return n.handler
		} else if len(path) == len(n.path) {
			if path == n.path {
				handlers = n.handler
			}
			return
		} else {
			// 如果传入的路径小于这个节点的路径，是传错了
			return nil
		}
	}
}
