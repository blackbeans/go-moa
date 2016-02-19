package core

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/blackbeans/turbo/packet"
	"strconv"
)

type RedisGetCodec struct {
	MaxFrameLength int
}

//读取数据
func (self RedisGetCodec) Read(reader *bufio.Reader) (*bytes.Buffer, error) {

	line, isPrefix, err := reader.ReadLine()
	if nil != err {
		return nil, errors.New("Read Packet Err " + err.Error())
	}

	//*$2\r\n$3\r\nGET\r\n${0}\r\n{"method":"","service-uri":""}
	start := bytes.HasPrefix(line, []byte{'*'})
	if start {
		//获取到共有多少个\r\n
		param := [2]*bytes.Buffer{}
		for i := 0; i < 2; i++ {

			//读取数组长度和对应的值
			tmp := bytes.NewBuffer(make([]byte, 0, 32))
			for {
				line, isPrefix, err = reader.ReadLine()
				if nil != err {
					return nil, errors.New("Read Packet Err " + err.Error())
				}

				//没有读取完这个命令的字节继续读取
				_, er := tmp.Write(line[1:])
				if nil != err {
					return nil, errors.New("Write Packet Into Buff  Err " + er.Error())
				}
				//读取完这个命令的字节
				if !isPrefix {
					break
				} else {

				}
			}

			//获取到数据的长度，读取数据

			l, _ := strconv.ParseInt(tmp.String(), 10, 64)
			length := int(l)
			if length >= self.MaxFrameLength {
				return nil, errors.New(fmt.Sprintf("Too Large Packet %d/%d", length, self.MaxFrameLength))
			}
			param[i] = bytes.NewBuffer(make([]byte, 0, length))
			dl := 0
			for {
				line, isPrefix, err = reader.ReadLine()
				if nil != err {
					return nil, errors.New("Read Packet Err " + err.Error())
				}

				//如果超过了给定的长度则忽略
				if (dl + len(line)) > length {
					return nil, errors.New(fmt.Sprintf("Invalid Packet Data %d/%d/%d ", i, (dl + len(line)), length))
				}

				//没有读取完这个命令的字节继续读取
				l, er := param[i].Write(line)
				if nil != err {
					return nil, errors.New("Write Packet Into Buff  Err " + er.Error())
				}
				//读取完这个命令的字节
				if !isPrefix {
					break
				} else {

				}
				dl += l
			}

		}
		//得到了get和数据将数据返回出去
		return param[1], nil
	} else {
		return nil, errors.New("Error Packet Prototol Is Not Get " + string(line[:0]))
	}
}

//反序列化
//包装为packet，但是头部没有信息
func (self RedisGetCodec) UnmarshalPacket(buff *bytes.Buffer) (*packet.Packet, error) {
	return packet.NewPacket(0, buff.Bytes()), nil
}

//序列化
//直接获取data
//$+n+\r\n+ [data]+\r\n
func (self RedisGetCodec) MarshalPacket(packet *packet.Packet) []byte {
	l := strconv.Itoa(len(packet.Data))
	buff := bytes.NewBuffer(make([]byte, 0, 1+len(l)+2+len(packet.Data)+2))
	buff.WriteString("$")
	buff.WriteString(l)
	buff.WriteString("\r\n")
	buff.WriteString(string(packet.Data))
	buff.WriteString("\r\n")
	return buff.Bytes()
}
