package websocket

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strings"
)

var (
	ErrNoMask         = errors.New("info no set mask")
	ErrOfWrongOrigin  = errors.New("the origin is wrong")
	ErrOfWrongConn    = errors.New("the connection is wrong")
	ErrOfWrongUpgrade = errors.New("the upgrade set is wrong")
	ErrOfWrongMethod  = errors.New("the request method is wrong")
	ErrOfWrongVersion = errors.New("the Sec-Websocket-Version is not 13")
	ErrOfClose        = errors.New("the conn is close")
)

type Handler func(a ...interface{}) error

type Msg struct {
	Typ     int
	Content []byte
}

type MyConn struct {
	conn         net.Conn
	status       bool
	ReadLimit    int
	WriteLimit   int
	ReadTimeout  chan error
	WriteTimeout chan error
	PongHandle   Handler
	IsPing       bool
}

type Upgrader struct {
	ReadBufferSize  int
	WriteBufferSize int
	CheckOrigin     func(r *http.Request) bool
}

func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (conn MyConn, err error) {
	// 创建一个MyConn
	conn = MyConn{
		conn:         nil,
		status:       false,
		ReadLimit:    u.ReadBufferSize,
		WriteLimit:   u.WriteBufferSize,
		ReadTimeout:  nil,
		WriteTimeout: nil,
		PongHandle:   nil,
		IsPing:       false,
	}

	//检查请求头 Connection
	if r.Header.Get("Connection") != "Upgrade" {
		err = ErrOfWrongConn
		return
	}
	//检查请求头 Upgrade
	if r.Header.Get("Upgrade") != "websocket" {
		err = ErrOfWrongUpgrade
		return
	}
	//检查请求方式
	if r.Method != http.MethodGet {
		err = ErrOfWrongMethod
		return
	}

	//检查请求头 Sec-Websocket-Version 是否为13
	if r.Header.Get("Sec-Websocket-Version") != "13" {
		err = ErrOfWrongVersion
		return
	}

	//检查Origin是否是允许的
	if u.CheckOrigin != nil && !u.CheckOrigin(r) {
		err = ErrOfWrongOrigin
		return
	}

	//检查请求头 Sec-Websocket-Key
	var keyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
	nKey := r.Header.Get("Sec-Websocket-Key")
	sha := sha1.New()
	sha.Write([]byte(nKey))
	sha.Write(keyGUID)
	key := base64.StdEncoding.EncodeToString(sha.Sum(nil))

	// 处理 Sec-Websocket-Protocol 子协议字段
	// 这里处理就默认选第一个 就不设置随机数了
	var webProtocol = ""
	if webProtocol = r.Header.Get("Sec-Websocket-Protocol"); webProtocol != "" {
		protocolSlice := strings.Split(webProtocol, ",")
		webProtocol = protocolSlice[0]
	}

	// 处理协议拓展 这里也取第一个
	var extendChose = ""
	if responseHeader != nil {
		for _, i := range responseHeader {
			extendChose = i[0]
			break
		}
	}

	// 从http.ResponseWriter重新拿到conn
	//调用 http.Hijacker 拿到这个连接现在开始就可以使用websocket通信了
	h, ok := w.(http.Hijacker)
	if !ok {
		err = errors.New("fail to hijacker the request")
		return
	}
	// 截获请求，建立websocket通信
	conn.conn, _, err = h.Hijack()
	if err != nil {
		return
	}

	// 回复报文 一系列请求头
	var resp []byte
	resp = append(resp, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: "...)
	resp = append(resp, key+"\r\n"...)
	if webProtocol != "" {
		resp = append(resp, "Sec-WebSocket-Protocol: "+webProtocol+"\r\n"...)
	}
	if extendChose != "" {
		resp = append(resp, "Sec-WebSocket-Extensions: "+extendChose+"\r\n"...)
	}
	resp = append(resp, "\r\n"...)

	//将请求报文写入
	_, err = conn.conn.Write(resp)
	conn.status = true
	return
}
