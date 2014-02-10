package bencoderpc

import (
	"errors"
	"github.com/3M3RY/go-bencode"
	"io"
	"net/rpc"
	"sync"
)

// Bug(emery): requests missing params are ignored by bencode.
var errMissingParams = errors.New("bencoderpc: request body missing params")

type serverCodec struct {
	dec *bencode.Decoder // for reading bencode values
	enc *bencode.Encoder // for writing bencode values
	c   io.Closer

	// temporary work space
	req serverRequest

	// bencode rpc clients can use arbitrary bencode values as request IDs.
	// Package rpc expects uint64 request IDs.
	// We assign uint64 sequence numbers to incoming requests
	// but save the original request ID in the pending map.
	// When rpc responds, we use the sequence number in
	// the response to find the original request ID.
	mutex   sync.Mutex // protects seq, pending
	seq     uint64
	pending map[uint64]*bencode.RawMessage //TODO benchmark this without a pointer
}

// NewServerCodec returns a new rpc.ServerCodec using bencode rpc on conn.
func NewServerCodec(conn io.ReadWriteCloser) rpc.ServerCodec {
	return &serverCodec{
		dec:     bencode.NewDecoder(conn),
		enc:     bencode.NewEncoder(conn),
		c:       conn,
		pending: make(map[uint64]*bencode.RawMessage),
	}
}

type serverRequest struct {
	Id     *bencode.RawMessage `bencode:"i"`
	Method string              `bencode:"m"`
	Params *bencode.RawMessage `bencode:"p"`
}

func (r *serverRequest) reset() {
	r.Id = nil
	r.Method = ""
	r.Params = nil
}

type serverResponse struct {
	Error  string              `bencode:"e"` //,omitempty"`
	Id     *bencode.RawMessage `bencode:"i"`
	Result interface{}         `bencode:"r"`
}

func (c *serverCodec) ReadRequestHeader(r *rpc.Request) error {
	c.req.reset()
	if err := c.dec.Decode(&c.req); err != nil {
		return err
	}

	r.ServiceMethod = c.req.Method

	// keep a seperate internal id
	c.mutex.Lock()
	c.seq++
	if c.req.Id != nil {
		c.pending[c.seq] = c.req.Id
	}
	c.req.Id = nil
	r.Seq = c.seq
	c.mutex.Unlock()

	return nil
}

func (c *serverCodec) ReadRequestBody(x interface{}) error {
	if x == nil {
		return nil
	}
	if c.req.Params == nil {
		return errMissingParams
	}
	return bencode.Unmarshal(*c.req.Params, x)
}

var zero = bencode.RawMessage([]byte("i0e"))

func (c *serverCodec) WriteResponse(r *rpc.Response, x interface{}) error {
	var resp serverResponse
	c.mutex.Lock()
	b, ok := c.pending[r.Seq]
	if !ok {
		c.mutex.Unlock()
		return errors.New("invalid sequence number in response")
	}
	delete(c.pending, r.Seq)
	c.mutex.Unlock()

	if b == nil {
		// Invalid request so no id.  Use bencode 0.
		b = &zero
	}
	resp.Id = b
	resp.Result = x
	if r.Error != "" {
		resp.Error = r.Error
	}
	return c.enc.Encode(resp)
}

func (c *serverCodec) Close() error {
	return c.c.Close()
}

// ServeConn runs the bencode rpc server on a single connection.
// ServeConn blocks, serving the connection until the client hangs up.
// The caller typically invokes ServeConn in a go statement.
func ServeConn(conn io.ReadWriteCloser) {
	rpc.ServeCodec(NewServerCodec(conn))
}
