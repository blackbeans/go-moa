package core

import (
	"context"
	"encoding/json"
	"github.com/blackbeans/turbo"
	"github.com/golang/snappy"
	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
	"time"
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
func (self BinaryCodec) UnmarshalPayload(p *turbo.Packet) (interface{}, error) {
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
			return turbo.Packet{}, err
		}
		if req.CreateTime <= 0 {
			req.CreateTime = time.Now().UnixNano() / int64(time.Millisecond)
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
			return p, err
		}
		p.PayLoad = *resp
	}

	return p.PayLoad, nil
}

func (self BinaryCodec) MarshalPayload(p *turbo.Packet) ([]byte, error) {

	var rawPayload []byte
	if p.Header.CmdType == REQ {
		data, err := json.Marshal(p.PayLoad)
		if nil != err {
			return nil, err
		}
		rawPayload = data

	} else if p.Header.CmdType == PING || p.Header.CmdType == PONG {
		//pong协议
		rawPayload, _ = json.Marshal(p.PayLoad)
	} else if p.Header.CmdType == RESP {

		resp, ok := p.PayLoad.(MoaRespPacket)
		if !ok {
			resp = MoaRespPacket{ErrCode: CODE_SERIALIZATION_SERVER,
				Message: "Invalid PayLoad Type Not MoaRespPacket"}
		}
		data, err := json.Marshal(resp)
		if nil != err {
			log.Errorf("BinaryCodec|MarshalPacket|Marshal|FAIL|%v", err)
			resp = MoaRespPacket{ErrCode: CODE_SERIALIZATION_SERVER,
				Message: "Invalid PayLoad Type Not MoaRespPacket"}
			data, _ = json.Marshal(resp)
		}
		rawPayload = data
	}

	return rawPayload, nil

}

type PiPo struct {
	Timestamp int64 `json:"timestamp"`
}

type MoaReqPacket struct {
	ServiceUri string `json:"action"`
	Params     struct {
		Method string        `json:"m"`
		Args   []interface{} `json:"args"`
	} `json:"params"`
	Properties map[string]string `json:"props,omitempty"`
	CreateTime int64             `json:"-"` //创建时间 ms
	Timeout    time.Duration     `json:"-"`
}

//moa请求协议的包
type MoaRawReqPacket struct {
	ServiceUri string `json:"action"`
	Params     struct {
		Method string            `json:"m"`
		Args   []json.RawMessage `json:"args"`
	} `json:"params"`
	Properties map[string]string `json:"props,omitempty"`
	CreateTime int64             `json:"-"` //创建时间 ms
	Timeout    time.Duration     `json:"-"`
	Source     string            `json:"-"`
}

//moa响应packet
type MoaRespPacket struct {
	ErrCode    int         `json:"ec"`
	Message    string      `json:"em"`
	CreateTime int64       `json:"-"` //创建时间 ms
	Result     interface{} `json:"result"`
}

//moa响应packet
type MoaRawRespPacket struct {
	ErrCode    int             `json:"ec"`
	Message    string          `json:"em"`
	CreateTime int64           `json:"-"` //创建时间 ms
	Result     json.RawMessage `json:"result"`
}

func Wrap2MoaRawRequest(data []byte) (*MoaRawReqPacket, error) {
	var req MoaRawReqPacket
	err := json.Unmarshal(data, &req)
	if nil != err {
		return nil, err
	} else {
		return &req, nil
	}

}

func Wrap2MoaRawResponse(data []byte) (*MoaRawRespPacket, error) {
	var resp MoaRawRespPacket
	err := json.Unmarshal(data, &resp)
	if nil != err {
		return nil, err
	}
	return &resp, nil
}

const (
	KEY_MOA_PROPERTIES = "moa.props"

	//MOA节点选择hash值
	KEY_MOA_PROPERTY_HASHID = "hashid"

	//MOA的调用环境，可以一直带到整个调用链结束
	KEY_MOA_PROPERTY_ENV_PRE = "moa.env.pre"
)

//切记切记。在使用完之后要做移除。否则会造成内存泄露
//调用 DetachGoProperties
// 注：如果要修改 moa context 中信息的存储方式，需要同时修改下面的 GetSpanCtx 和 WithSpanCtx
func AttachMoaProperty(ctx context.Context, key, val string) context.Context {

	props := ctx.Value(KEY_MOA_PROPERTIES)
	if nil != props {
		if v, ok := props.(map[string]string); ok {
			v[key] = val
			return ctx
		}
	}
	prop := make(map[string]string)
	prop[key] = val
	return context.WithValue(ctx, KEY_MOA_PROPERTIES, prop)
}

//剔除属性
func DetachMoaProperty(ctx context.Context, key string) {
	props := ctx.Value(KEY_MOA_PROPERTIES)
	if nil != props {
		if v, ok := props.(map[string]string); ok {
			delete(v, key)
		}
	}
}

//获取moa的上下文属性
func GetMoaProperty(ctx context.Context, key string) (string, bool) {
	props := ctx.Value(KEY_MOA_PROPERTIES)
	if nil != props {
		if v, ok := props.(map[string]string); ok {
			val, exist := v[key]
			return val, exist
		}
	}
	return "", false
}

// 从我们的 Context 中获取 SpanContext，如果没有则返回 nil
// 其实是从 context 的 moa.props 中获取信息
func GetSpanCtx(ctx context.Context) opentracing.SpanContext {
	props := ctx.Value(KEY_MOA_PROPERTIES)
	if props != nil {
		if v, ok := props.(map[string]string); ok {
			spanCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, opentracing.TextMapCarrier(v))
			if err != nil {
				return nil
			}
			return spanCtx
		}
	}
	return nil
}

// 将 SpanContext 存到我们的 Context 中，进行传递
// 其实是将一个键值对设置到了 context 的 moa.props 中
func WithSpanCtx(ctx context.Context, spCtx opentracing.SpanContext) context.Context {
	props := ctx.Value(KEY_MOA_PROPERTIES)
	var p map[string]string
	if props != nil {
		if v, ok := props.(map[string]string); ok {
			p = v
		} else {
			// KEY_MOA_PROPERTIES 中不是 map[string]string
			p = make(map[string]string)
		}
	} else {
		p = make(map[string]string)
	}
	opentracing.GlobalTracer().Inject(spCtx, opentracing.TextMap, opentracing.TextMapCarrier(p))
	return context.WithValue(ctx, KEY_MOA_PROPERTIES, p)
}
