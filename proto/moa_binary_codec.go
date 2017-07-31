package proto

import (
	"encoding/json"
	"fmt"

	"github.com/blackbeans/log4go"

	"github.com/blackbeans/turbo/packet"
)

const (
	REQ  = byte(0x01)
	RESP = byte(0x02)
	PING = byte(0x03)
	PONG = byte(0x04)
	INFO = byte(0x05)
)

type BinaryCodec struct {
	MaxFrameLength int
}

//反序列化
//包装为packet，但是头部没有信息
func (self BinaryCodec) UnmarshalPacket(p packet.Packet) (*packet.Packet, error) {

	if p.Header.CmdType == REQ {
		req, err := Wrap2MoaRawRequest(p.Data)
		if nil != err {
			return nil, err
		}
		p.PayLoad = *req
	} else if p.Header.CmdType == RESP {
		fmt.Printf("------%v\n", p)
	}

	return &p, nil
}

func (self BinaryCodec) MarshalPacket(p packet.Packet) ([]byte, error) {

	if p.Header.CmdType == REQ {
		data, err := json.Marshal(p.PayLoad)
		if nil != err {
			return nil, err
		}
		p.Data = data

	} else if p.Header.CmdType == RESP {

		resp, ok := p.PayLoad.(MoaRespPacket)

		if ok {
			var data []byte
			if v, ok := resp.Result.(string); ok {
				data = []byte(v)
			} else {
				d, err := json.Marshal(resp.Result)
				if nil != err {
					log4go.ErrorLog("codec", "BinaryCodec|MarshalPacket|Marshal|FAIL", err)
					return nil, err
				}
				data = d
			}
			p.Data = data
		} else {
			return nil, fmt.Errorf("Invalid Resp MoaRespPacket")
		}
	}
	resp := p.Marshal()
	return resp, nil

}
