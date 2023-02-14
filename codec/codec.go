package codec

import "io"

type Header struct {
	ServiceMethod string
	Seq           uint64
	Error         error
}

type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Writer(*Header, interface{}) error
}

const (
	GobType  = "Application/gob"
	JsonType = "Application/json"
)

type Type string

type NewCodecFunc func(writer io.ReadWriteCloser) Codec

var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}
