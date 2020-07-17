package streams

import (
	"fmt"
	"io"

	"wonderland.org/geneos/xmlrpc"
)

type Stream struct {
	io.Writer
	io.StringWriter
	xmlrpc.Sampler
	name string
}

// Sampler - wrap calls to xmlrpc
func Sampler(url string, entityName string, samplerName string) (s Stream, err error) {
	sampler, err := xmlrpc.NewClient(url, entityName, samplerName)
	s = Stream{}
	s.Sampler = sampler
	return
}

func (s *Stream) SetStreamName(name string) {
	s.name = name
}

func (s Stream) Write(data []byte) (n int, err error) {
	if s.name == "" {
		return 0, fmt.Errorf("streamname not set")
	}
	err = s.WriteMessage(s.name, string(data))
	if err != nil {
		return 0, err
	}
	n = len(data)
	return
}

func (s Stream) WriteString(data string) (n int, err error) {
	if s.name == "" {
		return 0, fmt.Errorf("streamname not set")
	}
	err = s.WriteMessage(s.name, data)
	if err != nil {
		return 0, err
	}
	n = len(data)
	return
}
