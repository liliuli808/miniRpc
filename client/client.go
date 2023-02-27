package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"miniRpc/codec"
	"miniRpc/server"
	"net"
	"sync"
)

type Client struct {
	seq      uint64
	cc       codec.Codec
	option   *server.Option
	sending  sync.Mutex
	header   codec.Header
	mu       sync.Mutex
	pending  map[uint64]*Call
	closing  bool
	shutdown bool
}

var ErrShutdown = errors.New("connection is shut down")

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.shutdown || !client.closing
}

func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing || client.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return client.seq, nil
}

func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

func (client *Client) terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

func (client *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err := client.cc.ReadBody(&h); err != nil {
			break
		}
		call := client.removeCall(client.seq)
		switch {
		case call == nil:
			err = client.cc.ReadBody(nil)
		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default:
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	client.terminateCalls(err)
}

func NewClient(conn net.Conn, opt *server.Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}
	err := json.NewEncoder(conn).Encode(opt)
	if err != nil {
		return nil, err
	}
	return newClientCodec(f(conn), opt)
}

func newClientCodec(cc codec.Codec, opt *server.Option) (*Client, error) {
	client := &Client{
		seq:     1,
		cc:      cc,
		option:  opt,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client, nil
}

func Dial(netWork, address string, option *server.Option) (client *Client, err error) {
	conn, err := net.Dial(netWork, address)
	if err != nil {
		return nil, err
	}

	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()

	return NewClient(conn, option)
}

func (client *Client) send(call *Call) {
	client.sending.Lock()
	defer client.sending.Unlock()

}
