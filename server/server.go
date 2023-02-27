package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"miniRpc/codec"
	"net"
	"reflect"
	"sync"
)

const MagicNum = 0x3bef5c

type Option struct {
	MagicNum  int
	CodecType codec.Type
}

var DefaultOption = &Option{
	CodecType: codec.GobType,
	MagicNum:  MagicNum,
}

type Server struct {
	mx *sync.Mutex
	wg *sync.WaitGroup
}

var DefaultServer = newServer()

func newServer() *Server {
	return &Server{
		mx: new(sync.Mutex),
		wg: new(sync.WaitGroup),
	}
}

func (server *Server) Accept(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go server.ServerConn(conn)
	}
}

func Accept(lis net.Listener) { DefaultServer.Accept(lis) }

func (server *Server) ServerConn(conn io.ReadWriteCloser) {
	defer conn.Close()
	var opt Option
	err := json.NewDecoder(conn).Decode(&opt)
	if err != nil {
		return
	}
	if opt.MagicNum != MagicNum {
		log.Printf("rpc server: invalid magic number %x", opt.MagicNum)
		return
	}
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}
	server.serverCodec(f(conn))
}

var invalidRequest = struct{}{}

func (server *Server) serverCodec(cc codec.Codec) {
	for {
		req, err := server.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.header.Error = err.Error()
			server.sendRequest(cc, req.header, invalidRequest)
			continue
		}
		server.wg.Add(1)
		go server.handelRequest(cc, req)
	}
	server.wg.Wait()
	cc.Close()
}

type request struct {
	header       *codec.Header
	argv, replyV reflect.Value
}

func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := request{header: h}
	req.argv = reflect.New(reflect.TypeOf(""))
	if err := cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server: read argv err:", err)
	}
	return &req, nil
}

func (server *Server) sendRequest(cc codec.Codec, h *codec.Header, body interface{}) {
	server.mx.Lock()
	defer server.mx.Unlock()
	err := cc.Writer(h, body)
	if err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

func (server *Server) handelRequest(cc codec.Codec, req *request) {
	defer server.wg.Done()
	log.Println(req.header, req.argv.Elem())
	req.replyV = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.header.Seq))
	server.sendRequest(cc, req.header, req.replyV.Interface())

}
