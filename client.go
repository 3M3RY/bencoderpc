package bencoderpc

import (
	"github.com/3M3RY/go-bencode"
	"io"
	"net"
	"net/rpc"
	"sync"
)

type clientCodec struct {
	dec *bencode.Decoder // for reading bencode values
	enc *bencode.Encoder // for writing bencode values
	c   io.Closer

	// temporary work space
	req  clientRequest
	resp clientResponse

	// bencode RPC responses include the request id but not the request method.
	// Package rpc expects both.
	// We save the request method in pending when sending a request
	// and then look it up by request ID when filling out the rpc Response.
	mutex   sync.Mutex        // protects pending
	pending map[uint64]string // map request id to method name
}

// NewClientCodec returns a new rpc.ClientCodec using bencode RPC on conn.
func NewClientCodec(conn io.ReadWriteCloser) rpc.ClientCodec {
	return &clientCodec{
		dec:     bencode.NewDecoder(conn),
		enc:     bencode.NewEncoder(conn),
		c:       conn,
		pending: make(map[uint64]string),
	}
}

type clientRequest struct {
	Id     uint64      `bencode:"i"`
	Method string      `bencode:"m"`
	Params interface{} `bencode:"p"`
}

func (c *clientCodec) WriteRequest(r *rpc.Request, params interface{}) error {
	c.mutex.Lock()
	c.pending[r.Seq] = r.ServiceMethod
	c.mutex.Unlock()
	c.req.Id = r.Seq
	c.req.Method = r.ServiceMethod
	c.req.Params = params
	return c.enc.Encode(&c.req)
}

type clientResponse struct {
	Error  string              `bencode:"e"`
	Id     uint64              `bencode:"i"`
	Result *bencode.RawMessage `bencode:"r"`
}

func (r *clientResponse) reset() {
	r.Error = ""
	r.Id = 0
	r.Result = nil
}

func (c *clientCodec) ReadResponseHeader(r *rpc.Response) error {
	c.resp.reset()

	if err := c.dec.Decode(&c.resp); err != nil {
		return err
	}

	c.mutex.Lock()
	r.ServiceMethod = c.pending[c.resp.Id]
	delete(c.pending, c.resp.Id)
	c.mutex.Unlock()

	r.Seq = c.resp.Id
	if c.resp.Error != "" {
		r.Error = c.resp.Error
	}
	return nil
}

func (c *clientCodec) ReadResponseBody(x interface{}) error {
	if x == nil {
		return nil
	}
	return bencode.Unmarshal(*c.resp.Result, x)
}

func (c *clientCodec) Close() error {
	return c.c.Close()
}

// NewClient returns a new rpc.Client to handle requests to the
// set of services at the other end of the connection.
func NewClient(conn io.ReadWriteCloser) *rpc.Client {
	return rpc.NewClientWithCodec(NewClientCodec(conn))
}

// Dial connects to a bencode RPC server at the specified network address.
func Dial(network, address string) (*rpc.Client, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return NewClient(conn), err
}
