package proto

import (
	"encoding/json"

	"github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"
	"github.com/golang/snappy"
)

const (
	REQ  = byte(0x01)
	RESP = byte(0x02)
	PING = byte(0x03)
	PONG = byte(0x04)
	INFO = byte(0x05)
)

const (
	COMPRESS_SNAPPY = 0x01 //snappy算法
)

type BinaryCodec struct {
	MaxFrameLength int
	SnappyCompress bool
}

//snappy解压缩
func Decompress(src []byte) ([]byte, error) {
	l, err := snappy.DecodedLen(src)
	if nil != err {
		return nil, err
	}
	if l%256 != 0 {
		l = (l/256 + 1) * 256
	}
	dest := make([]byte, l)
	decompressData, err := snappy.Decode(dest, src)
	return decompressData, err
}

//snapp压缩
func Compress(src []byte) []byte {
	l := snappy.MaxEncodedLen(len(src))
	if l%256 != 0 {
		l = (l/256 + 1) * 256
	}

	dest := make([]byte, l)
	compressData := snappy.Encode(dest, src)
	return compressData
}

//反序列化
//包装为packet，但是头部没有信息
func (self BinaryCodec) UnmarshalPacket(p turbo.Packet) (*turbo.Packet, error) {

	useSnappy := p.Header.Extension & COMPRESS_SNAPPY
	//使用snap
	if useSnappy == COMPRESS_SNAPPY {
		d, err := Decompress(p.Data)
		if nil != err {
			return nil, err
		}
		p.Data = d
	}

	if p.Header.CmdType == REQ {
		//req
		req, err := Wrap2MoaRawRequest(p.Data)
		if nil != err {
			return nil, err
		}
		p.PayLoad = *req
	} else if p.Header.CmdType == PING || p.Header.CmdType == PONG {
		//ping
		var ping PiPo
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

func (self BinaryCodec) MarshalPacket(p turbo.Packet) ([]byte, error) {

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

		data, err := json.Marshal(resp)
		if nil != err {
			log4go.ErrorLog("codec", "BinaryCodec|MarshalPacket|Marshal|FAIL", err)
			resp = MoaRespPacket{ErrCode: CODE_SERIALIZATION_SERVER,
				Message: "Invalid PayLoad Type Not MoaRespPacket"}
			data, _ = json.Marshal(resp)
		}
		p.Data = data
	}

	//使用snap
	if self.SnappyCompress {
		//设置snappy压缩
		p.Header.Extension = (p.Header.Extension | COMPRESS_SNAPPY)
		d := Compress(p.Data)
		p.Data = d
	}
	resp := p.Marshal()
	return resp, nil

}
