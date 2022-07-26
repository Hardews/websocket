package main

import "strings"

type Hub struct {
	clients map[*Client]bool

	broadcast chan []byte

	register chan *Client

	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				if strings.Contains(client.username, "[房主]") { //如果写了登陆注册就会是token字段的验证，会更安全
					delete(Room, client.roomName)
					for c, _ := range h.clients {
						delete(h.clients, c)
						close(c.send)
					}
				} else {
					delete(h.clients, client)
					close(client.send)
				}
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
