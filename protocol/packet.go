package protocol

import (
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
	ErrCode int         `json:"errorCode"`
	Message string      `json:"message"`
	Result  interface{} `json:"result"`
}

func Command2MoaRequest(cr CommandRequest) MoaReqPacket {
	req := MoaReqPacket{}
	req.ServiceUri = cr.ServiceUri
	req.Method = cr.Params.Method
	req.Params = cr.Params.Args
	return req
}
