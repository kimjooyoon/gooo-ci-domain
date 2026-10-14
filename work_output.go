package cibudgetdomain

import (
	"hash"
	"io"
	"sync"
)

// The full stream is hashed and forwarded. It does not grow a local log buffer.
type workOutput struct {
	sync.Mutex
	sink    io.Writer
	hash    hash.Hash
	failure error
	bytes   int64
}

func (output *workOutput) Write(data []byte) (int, error) {
	output.Lock()
	defer output.Unlock()
	_, _ = output.hash.Write(data)
	output.bytes += int64(len(data))
	n, err := output.sink.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		output.failure = err
	}
	return n, err
}
