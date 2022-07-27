package main

import (
	"gin"
	"net/http"
	"websocket"

	"log"
	"math/rand"
	"time"
)

const (
	writeWait = 10 * time.Second

	pongWait = 60 * time.Second

	pingPeriod = (pongWait * 9) / 10

	maxMessageSize = 512
)

var Room = make(map[string]*Hub)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	roomName string

	username string

	hub *Hub

	conn websocket.MyConn

	send chan []byte
}

func NewRoom(ctx *gin.Context) {
	hub := newHub()
	go hub.run()

	// 没有前端所以测试直接是固定房间名字
	roomName := "Hardews"
	Room[roomName] = hub

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Fatalln(err)
	}

	username := "Hardews"

	client := &Client{
		roomName: roomName,
		username: "[房主]" + username,
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 1024),
	}

	hub.register <- client

	go client.readPump()
	go client.writePump()
}

func ShowRoom(ctx *gin.Context) {
	if len(Room) == 0 {
		ctx.JSON(200, gin.H{
			"msg": "have no room now",
		})
	}
	for s, hub := range Room {
		ctx.JSON(200, gin.H{
			"room": s,
		})
		var names []string
		for client, _ := range hub.clients {
			names = append(names, client.username)
		}
		ctx.JSON(200, gin.H{
			"成员": names,
		})
	}
}

func JoinRoom(ctx *gin.Context) {
	roomName := "Hardews"
	hub := Room[roomName]
	go hub.run()

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Fatalln(err)
	}

	username := randomName(6)

	client := &Client{
		username: username,
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 1024),
	}

	hub.register <- client

	go client.readPump()
	go client.writePump()
}

var letterRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWSYZabcdefghijklmnopqrstuvwsyz1234567890")

// randomName 生成随机字符串方便测试
func randomName(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close(websocket.CloseUnsupported, "do not know")
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadLine(pongWait)
	// 心跳 所以当收到pong消息时 把时间延长 就可以维持连线
	c.conn.SetPongHandler(func(a ...interface{}) error { c.conn.SetReadDeadLine(pongWait); return nil })
	for {
		message, err := c.conn.ReadMsg()
		if err != nil {
			log.Println(err)
			break
		}

		c.hub.broadcast <- []byte(c.username + "说: " + string(message.Content))
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close(websocket.CloseUnsupported, "do not know")
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadLine(writeWait)
			if !ok {
				// 用户关闭连接
				c.conn.Close(websocket.CloseNormal, "the user close the connection")
				return
			}

			c.conn.WriteMsg(websocket.Msg{
				Typ:     websocket.TextMessage,
				Content: message,
			})

		case <-ticker.C:
			// 心跳 发送一个ping 服务端应发送pong消息
			c.conn.SetWriteDeadLine(writeWait)
			if err := c.conn.WriteMsg(websocket.Msg{
				Typ:     websocket.PingMessage,
				Content: []byte("PING"),
			}); err != nil {
				c.conn.Close(websocket.CloseGoingAway, "time out")
				return
			}
		}
	}
}
