package framework_sets

import (
	"errors"
	"log"
	"time"
)

const (
	TextMessage   = 1
	BinaryMessage = 2
	CloseMessage  = 8
	PingMessage   = 9
	PongMessage   = 10
)

var (
	CloseNormal          = []byte{0x03, 0xe8} // 1000 表示正常关闭
	CloseGoingAway       = []byte{0x03, 0xe9} // 1001
	CloseUnsupported     = []byte{0x03, 0xeb} // 1003
	ClosePolicyViolation = []byte{0x03, 0xf0} // 1008 表示收到了不符合约定的数据
	CloseTooLarge        = []byte{0x03, 0xf1} // 1009 表示收到的数据帧过大，不符合设定
)

var (
	ErrOfReadSizeNoAllow  = errors.New("the size of read is no allow")
	ErrOfWriteSizeNoAllow = errors.New("the size of write is no allow")
	ErrOfBadPayloadLen    = errors.New("bad payload len")
	ErrOfNoMean           = errors.New("the chose do not mean")
)

func (c *MyConn) ReadMsg() (m Msg, err error) {
	// 根据数据帧读取数据
	if !c.status {
		return m, ErrOfClose
	}

	var msg []byte

	// 这个标签是处理消息分片用的
again:

	// 读取第一个字节
	firstByte := make([]byte, 1)
	_, err = c.conn.Read(firstByte)
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
		c.Close(ClosePolicyViolation, err.Error())
		return
	}

	// 读取第二个字节
	secondByte := make([]byte, 1)
	_, err = c.conn.Read(secondByte)
	if err != nil {
		return
	}

	// 处理payload len
	// 0111 1111 去掉第一位mask的值
	payloadLen := int(secondByte[0] & 0x7f)

	// 右移七位获得最高位
	mask := int(secondByte[0] >> 7)
	// 检查是否使用mask
	if mask != 1 {
		err = ErrNoMask
		return
	}

	// 这里是处理payload len
	switch {
	case 125 >= payloadLen && payloadLen > 0:
	case payloadLen == 126:
		// 处理后两个字节
		payloadLen = 0
		payloadLenByte := make([]byte, 2)
		_, err = c.conn.Read(payloadLenByte)
		if err != nil {
			return
		}
		payloadLen = int(payloadLenByte[0])<<8 + int(payloadLenByte[1])
	case payloadLen == 127:
		// 处理后八个字节
		payloadLen = 0
		payloadLenByte := make([]byte, 8)
		_, err = c.conn.Read(payloadLenByte)
		if err != nil {
			return
		}
		for i := 7; i >= 0; i++ {
			if payloadLenByte[i] == 0 {
				continue
			}
			payloadLen += int64(payloadLenByte[i]) << (i * 8)
		}
	default:
		err = ErrOfBadPayloadLen
		return
	}

	// 先判断是否设置，再判断这次读取数据的大小是否超过设置数量
	if c.ReadLimit != 0 {
		if payloadLen > c.ReadLimit {
			err = ErrOfReadSizeNoAllow
			c.Close(CloseTooLarge, err.Error())
			return
		}
	}

	// 接下来的四个字节为mask key
	maskKey := make([]byte, 4)
	_, err = c.conn.Read(maskKey)
	if err != nil {
		return
	}

	// 读取payload
	payload := make([]byte, payloadLen)
	_, err = c.conn.Read(payload)
	if err != nil {
		return
	}

	//ping pong 消息是心跳消息
	m.Typ = int(opcode)
	switch opcode {
	case PingMessage:
		// 按照用户设置的函数执行
		err = c.PingHandle()
		if err != nil {
			return
		}
		goto again
	case PongMessage:
		// 因为有时候客户端可能无缘无故发送pong，需要忽略
		if !c.IsPing {
			goto again
		}
		c.IsPing = false
		// 按照用户设置的函数执行
		err = c.PongHandle()
		if err != nil {
			return
		}
		// 执行完用户设置的函数后就应该返回再次读取
		goto again
	case TextMessage:
		// 因为这里msg是用append方法拼装，如果遇到消息分片重来一次就没有影响
		for i := 0; i < payloadLen; i++ {
			msg = append(msg, payload[i]^maskKey[i%4])
		}
		m.Content = msg
	case BinaryMessage:
		for i := 0; i < payloadLen; i++ {
			msg = append(msg, payload[i]^maskKey[i%4])
		}
		m.Content = msg
	case CloseMessage:
		var errMsg []byte
		for i := 0; i < payloadLen; i++ {
			errMsg = append(errMsg, payload[i]^maskKey[i%4])
		}
		// 关闭原因
		err = errors.New(string(errMsg[2:]))
		c.Close(CloseNormal, string(errMsg[2:]))
		c.status = false
	default:
		err = ErrOfNoMean
	}
	if fin == 0 {
		goto again
	}
	return
}

func (c *MyConn) WriteMsg(m Msg) (err error) {
	if !c.status {
		return ErrOfClose
	}
	// 按照数据帧写出数据
	// 消息内容
	data := m.Content

	length := len(data)
	if c.WriteLimit != 0 && m.Typ != CloseMessage {
		if length > c.WriteLimit {
			err = ErrOfWriteSizeNoAllow
			c.Close(CloseTooLarge, err.Error())
			return
		}
	}

	// 写出第一个字节
	switch m.Typ {
	case TextMessage:
		// FIN RSV opcode
		_, err = c.conn.Write([]byte{0x81})
	case BinaryMessage:
		_, err = c.conn.Write([]byte{0x82})
	case PingMessage:
		_, err = c.conn.Write([]byte{0x89})
		c.IsPing = true
	case PongMessage:
		_, err = c.conn.Write([]byte{0x8a})
	case CloseMessage:
		_, err = c.conn.Write([]byte{0x88})
	}

	// 处理payload len
	var tmp = 0
	var payLenByte byte
	switch {
	case length <= 125:
	case length > 125 && length <= 65535:
		tmp = length
		length = 126
	case length > 65535:
		tmp = length
		length = 127
	}

	// 用按位或运算来转换
	payLenByte = byte(0x00) | byte(length)
	_, err = c.conn.Write([]byte{payLenByte})

	if tmp != 0 {
		var lengthByte []byte
		switch length {
		case 126:
			// 这里处理len的想法是
			// 一个需要存储两个字节的len 假如是 11110000 11110000
			// 先让 length | 0x00 进行运算，取到length的低八位,append进切片
			// 然后让 length减去低八位之后,再右移八位，获取高八位,再重复进行上面的运算
			// 最后倒着写进去就好了 下面127同理
			for i := 0; i < 2; i++ {
				lengthTmp := byte(length) | byte(0x00)
				lengthByte = append(lengthByte, lengthTmp)
				length -= int(lengthTmp)
				lengthTmp = byte(length >> 8)
			}
			tmp := lengthByte[0]
			lengthByte[0] = lengthByte[1]
			lengthByte[1] = tmp
			_, err = c.conn.Write(lengthByte)
			if err != nil {
				return err
			}
		case 127:
			for i := 0; i < 8; i++ {
				lengthTmp := byte(length) | byte(0x00)
				lengthByte = append(lengthByte, lengthTmp)
				length -= int(lengthTmp)
				lengthTmp = byte(length >> 8)
			}
			var nLengthByte []byte
			for i := 7; i >= 0; i-- {
				nLengthByte = append(nLengthByte, lengthByte[i])
			}
			_, err = c.conn.Write(nLengthByte)
			if err != nil {
				return err
			}
		default:

		}
	}

	// 写出发送的数据
	_, err = c.conn.Write(data)
	return
}

func (c *MyConn) Close(closeCode []byte, closeReason string) {
	var m Msg
	m.Typ = CloseMessage
	m.Content = append(m.Content, closeCode[0])
	m.Content = append(m.Content, closeCode[1])           // 状态码
	m.Content = append(m.Content, []byte(closeReason)...) // 关闭原因 可以不具有可读性
	err := c.WriteMsg(m)
	if err != nil {
		log.Println(err)
	}
	c.conn.Close()
}

func (c *MyConn) SetWriteDeadLine(td time.Duration) error {
	return c.conn.SetWriteDeadline(time.Now().Add(td))
}

func (c *MyConn) SetReadDeadLine(td time.Duration) error {
	return c.conn.SetReadDeadline(time.Now().Add(td))
}

func (c *MyConn) SetPongHandler(pongTask func(a ...interface{}) error) {
	if pongTask() == nil {
		pongTask = func(a ...interface{}) error {
			return nil
		}
	}
	c.PongHandle = pongTask
}

func (c *MyConn) SetPingHandler(pongTask func(a ...interface{}) error) {
	if pongTask() == nil {
		pongTask = func(a ...interface{}) error {
			return nil
		}
	}
	c.PongHandle = pongTask
}

func (c *MyConn) SetReadLimit(size int) {
	c.ReadLimit = size
}

func (c *MyConn) SetWriteLimit(size int) {
	c.WriteLimit = size
}
