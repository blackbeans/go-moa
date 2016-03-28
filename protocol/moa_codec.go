package protocol

import (
	"bufio"
	"bytes"
	b "encoding/binary"
	// "errors"
	"fmt"
	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo/packet"
	"github.com/go-errors/errors"
	"strconv"
	"strings"
)

const (
	PADDING = 0x00
	GET     = 0x01
	PING    = 0x02
)

type RedisGetCodec struct {
	MaxFrameLength int
}

//读取数据
func (self RedisGetCodec) Read(reader *bufio.Reader) (*bytes.Buffer, error) {

	defer func() {
		if err := recover(); nil != err {
			er, ok := err.(*errors.Error)
			if ok {
				stack := er.ErrorStack()
				log.ErrorLog("moa-server", "RedisGetCodec|Read|ERROR|%s", stack)
			} else {
				log.ErrorLog("moa-server", "RedisGetCodec|Read|ERROR|%v", er)
			}
		}

	}()
	line, isPrefix, err := reader.ReadLine()
	if nil != err {
		return nil, errors.New("Read Command Args Count Packet Err " + err.Error())
	}
	//*1\r\n$4\r\nPING\r\n
	//*2\r\n$3\r\nGET\r\n${0}\r\n{"method":"","service-uri":""}\r\n
	start := bytes.HasPrefix(line, []byte{'*'})
	if start {
		argsCount, _ := strconv.ParseInt(string(line[1:len(line)]), 10, 64)
		ac := int(argsCount)

		//获取到共有多少个\r\n
		params := make([]*bytes.Buffer, 0, ac)
		for i := 0; i < ac; i++ {

			//读取数组长度和对应的值
			tmp := bytes.NewBuffer(make([]byte, 0, 32))
			for {
				line, isPrefix, err = reader.ReadLine()
				if nil != err {
					return nil, errors.New("Read Command Len Packet Err " + err.Error())
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
			//+4B+1B 是为了给将长度协议类型附加在dataBuff的末尾
			dataBuff := bytes.NewBuffer(make([]byte, 0, 4+length+1))
			b.Write(dataBuff, b.BigEndian, int32(length))
			dl := 0
			for {
				line, isPrefix, err = reader.ReadLine()
				if nil != err {
					return nil, errors.New("Read Command Data Packet Err " + err.Error())
				}

				//如果超过了给定的长度则忽略
				if (dl + len(line)) > length {
					return nil, errors.New(fmt.Sprintf("Invalid Packet Data %d:[%d/%d]\t%s ",
						i, (dl + len(line)), length, string(line)))
				}
				//没有读取完这个命令的字节继续读取
				l, er := dataBuff.Write(line)
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
			params = append(params, dataBuff)

		}

		cmdType := strings.ToUpper(string(params[0].Bytes()[4:]))
		//获取协议的类型
		switch cmdType {
		case "PING":
			params[ac-1].WriteByte(PING)
		case "GET":
			params[ac-1].WriteByte(GET)
		default:
			params[ac-1].WriteByte(PADDING)
		}

		//得到了get和Ping数据将数据返回出去
		return params[ac-1], nil
	} else {
		return nil, errors.New("Error Packet Prototol Is Not Get " + string(line))
	}
}

//反序列化
//包装为packet，但是头部没有信息
func (self RedisGetCodec) UnmarshalPacket(buff *bytes.Buffer) (*packet.Packet, error) {
	var l int32
	b.Read(buff, b.BigEndian, &l)
	d := buff.Bytes()
	return packet.NewPacket(d[l], d[:l]), nil
}

var ERROR = []byte("-Error message\r\n")

//序列化
//直接获取data
//GET $+n+\r\n+ [data]+\r\n
//PING +PONG
func (self RedisGetCodec) MarshalPacket(packet *packet.Packet) []byte {
	body := string(packet.Data)
	l := len(strconv.Itoa(len(body)))
	if packet.Header.CmdType == GET {

		buff := bytes.NewBuffer(make([]byte, 0, 1+l+2+len(body)+2))
		buff.WriteString("$")
		buff.WriteString(strconv.Itoa(len(body)))
		buff.WriteString("\r\n")
		buff.WriteString(body)
		buff.WriteString("\r\n")
		return buff.Bytes()
	} else if packet.Header.CmdType == PING {
		buff := bytes.NewBuffer(make([]byte, 0, 1+len(body)))
		buff.WriteString("+")
		buff.WriteString(body)
		buff.WriteString("\r\n")
		return buff.Bytes()
	}

	return ERROR
}
