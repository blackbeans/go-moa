package protocol

import (
	"encoding/json"
	"github.com/blackbeans/turbo/packet"
	"time"
)

type CommandRequest struct {
	ServiceUri string `json:"action"`
	Params     struct {
		Method string        `json:"m"`
		Args   []interface{} `json:"args"`
	} `json:"params"`
}

//moa请求协议的包
type MoaReqPacket struct {
	ServiceUri string        `json:"action"`
	Method     string        `json:"m"`
	Params     []interface{} `json:"args"`
	Timeout    time.Duration `json:"-"`
}

//moa响应packet
type MoaRespPacket struct {
	ErrCode int         `json:"ec"`
	Message string      `json:"em"`
	Result  interface{} `json:"result"`
}

func Command2MoaRequest(cr CommandRequest) MoaReqPacket {
	req := MoaReqPacket{}
	req.ServiceUri = cr.ServiceUri
	req.Method = cr.Params.Method
	req.Params = cr.Params.Args
	return req
}

func Wrap2MoaRequest(data []byte) (*MoaReqPacket, error) {
	var req CommandRequest
	err := json.Unmarshal(data, &req)
	if nil != err {
		return nil, err
	} else {
		mrp := Command2MoaRequest(req)
		return &mrp, nil
	}

}

func Wrap2ResponsePacket(p *packet.Packet, resp MoaRespPacket) (*packet.Packet, error) {
	data, err := json.Marshal(resp)
	respPacket := packet.NewRespPacket(p.Header.Opaque, p.Header.CmdType, data)
	return respPacket, err
}
