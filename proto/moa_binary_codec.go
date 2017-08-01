package proto

import (
	"encoding/json"

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
		//req
		req, err := Wrap2MoaRawRequest(p.Data)
		if nil != err {
			return nil, err
		}
		p.PayLoad = *req
	} else if p.Header.CmdType == PING || p.Header.CmdType == PONG {
		//ping
		var ping map[string]interface{}
		json.Unmarshal(p.Data, &ping)
		p.PayLoad = ping
	} else if p.Header.CmdType == RESP {
		//resp
		resp, err := Wrap2MoaRawResponse(p.Data)
		if nil != err {
			return nil, err
		}
		p.PayLoad = *resp
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

	} else if p.Header.CmdType == PING || p.Header.CmdType == PONG {
		//pong协议
		payLoad, _ := json.Marshal(p.PayLoad)
		p.Data = payLoad
	} else if p.Header.CmdType == RESP {

		resp, ok := p.PayLoad.(MoaRespPacket)
		if !ok {
			resp = MoaRespPacket{ErrCode: CODE_SERIALIZATION_SERVER,
				Message: "Invalid PayLoad Type Not MoaRespPacket"}
		}
		var data []byte
		if v, ok := resp.Result.(string); ok {
			data = []byte(v)
		} else {
			d, err := json.Marshal(resp)
			if nil != err {
				log4go.ErrorLog("codec", "BinaryCodec|MarshalPacket|Marshal|FAIL", err)
				resp = MoaRespPacket{ErrCode: CODE_SERIALIZATION_SERVER,
					Message: "Invalid PayLoad Type Not MoaRespPacket"}
				d, _ = json.Marshal(resp)
			}
			data = d
		}
		p.Data = data

	}
	resp := p.Marshal()
	return resp, nil

}
