package core

import (
	"bytes"
	"github.com/blackbeans/turbo/codec"
	"github.com/blackbeans/turbo/packet"
)

type RedisCodec struct {
	codec.LineBasedCodec
}

//反序列化
//包装为packet，但是头部没有信息
func (self RedisCodec) UnmarshalPacket(buff *bytes.Buffer) (*packet.Packet, error) {
	return packet.NewPacket(0, buff.Bytes()), nil
}

//序列化
//直接获取data
func (self RedisCodec) MarshalPacket(packet *packet.Packet) []byte {
	return packet.Data
}
