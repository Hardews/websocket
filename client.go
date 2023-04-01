package framework_sets

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

type ClientConn struct {
	body       io.ReadCloser
	ReadLimit  int
	PingHandle Handler
	PongHandle Handler
}

func NewClient(u *url.URL) (c *ClientConn, err error) {
	d := &Dialer{HandshakeTimeout: 45 * time.Second}
	return d.dial(u.String())
}

type Dialer struct {
	HandshakeTimeout time.Duration
}

func (d *Dialer) dial(urlStr string) (*ClientConn, error) {
	challengeKey := base64.StdEncoding.EncodeToString([]byte(randWebsocketKey()))

	URL, err := url.Parse(urlStr)
	if err != nil {
		return nil, errors.New("parse url failed,err:" + err.Error())
	}

	req := &http.Request{
		Method:     http.MethodGet,
		URL:        URL,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Host:       URL.Host,
	}

	req.Header["Upgrade"] = []string{"websocket"}
	req.Header["Connection"] = []string{"Upgrade"}
	req.Header["Sec-WebSocket-Key"] = []string{challengeKey}
	req.Header["Sec-WebSocket-Version"] = []string{"13"}

	var resp *http.Response
	var client = &http.Client{Timeout: d.HandshakeTimeout}
	resp, err = client.Do(req)
	if err != nil {
		return nil, errors.New("send req failed,err:" + err.Error())
	}

	if resp.StatusCode != 101 ||
		resp.Header.Get("Sec-WebSocket-Accept") != encodingKey(challengeKey) {
		return nil, errors.New("the response body is wrong")
	}

	// 通过读取body的内容读取服务端发送的信息
	var myConn = &ClientConn{body: resp.Body}
	return myConn, nil
}

var letterRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWSYZabcdefghijklmnopqrstuvwsyz1234567890")
var keyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func randWebsocketKey() string {
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, 16) // 16 字节
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func encodingKey(key string) string {
	sha := sha1.New()
	sha.Write([]byte(key))
	sha.Write(keyGUID)
	return base64.StdEncoding.EncodeToString(sha.Sum(nil))
}

func (c *ClientConn) ReadMsg() (m Msg, err error) {
	// 根据数据帧读取数据
	var msg []byte

again: // 这个标签是处理消息分片用的

	// 读取第一个字节
	firstByte := make([]byte, 1)
	_, err = c.body.Read(firstByte)
	if err != nil {
		return
	}
	fin := firstByte[0] >> 7
	opcode := firstByte[0] & 15 // 15 的二进制表示为 00001111 取后四位

	// 右移左边补0 再进行按位与运算 00000001 取最后一位
	RSV1 := firstByte[0] >> 6 & 1
	RSV2 := firstByte[0] >> 5 & 1
	RSV3 := firstByte[0] >> 4 & 1

	// 协议拓展判断
	if RSV1 != 0 || RSV2 != 0 || RSV3 != 0 {
		err = errors.New("rsv set")
		c.body.Close()
		return
	}

	// 读取第二个字节
	secondByte := make([]byte, 1)
	_, err = c.body.Read(secondByte)
	if err != nil {
		return
	}

	// 处理payload len
	// 0111 1111 去掉第一位mask的值
	payloadLen := int(secondByte[0] & 0x7f)

	// 右移七位获得最高位mask
	// 服务端发送无mask
	mask := int(secondByte[0] >> 7)
	if mask != 0 {
		err = errors.New("the info set mask")
		return
	}

	// 这里是处理payload len
	switch {
	case 125 >= payloadLen && payloadLen > 0:
	case payloadLen == 126:
		// 处理后两个字节
		payloadLen = 0
		payloadLenByte := make([]byte, 2)
		_, err = c.body.Read(payloadLenByte)
		if err != nil {
			return
		}
		for _, b := range payloadLenByte {
			payloadLen += int(b)
		}
	case payloadLen == 127:
		// 处理后八个字节
		payloadLen = 0
		payloadLenByte := make([]byte, 8)
		_, err = c.body.Read(payloadLenByte)
		if err != nil {
			return
		}
		for _, b := range payloadLenByte {
			payloadLen += int(b)
		}
	default:
		err = ErrOfBadPayloadLen
		return
	}

	// 先判断是否设置，再判断这次读取数据的大小是否超过设置数量
	if c.ReadLimit != 0 {
		if payloadLen > c.ReadLimit {
			err = ErrOfReadSizeNoAllow
			c.body.Close()
			return
		}
	}

	// 读取payload
	payload := make([]byte, payloadLen)
	_, err = c.body.Read(payload)
	if err != nil {
		return
	}

	//ping pong 消息是心跳消息
	m.Typ = int(opcode)
	switch opcode {
	case PingMessage:
		// 按照用户设置的函数执行
		err = c.PongHandle()
		goto again
	case PongMessage:
		err = c.PingHandle()
		goto again
	case TextMessage:
		// 因为这里msg是用append方法拼装，如果遇到消息分片重来一次就没有影响
		msg = append(msg, payload...)
		m.Content = msg
	case BinaryMessage:
		msg = append(msg, payload...)
		m.Content = msg
	case CloseMessage:
		msg = append(msg, payload...)
		m.Content = msg
		c.body.Close()
	default:
		err = ErrOfNoMean
	}
	if fin == 0 {
		goto again
	}
	return
}

func (c *ClientConn) Close() {
	c.body.Close()
}

func (c *ClientConn) SetPongHandler(pongTask func(a ...interface{}) error) {
	if pongTask() == nil {
		pongTask = func(a ...interface{}) error {
			return nil
		}
	}
	c.PongHandle = pongTask
}

func (c *ClientConn) SetReadLimit(size int) {
	c.ReadLimit = size
}
