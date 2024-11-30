package channel

type BoundedStream struct {
	stream chan []byte
	size   int64
}

func NewBoundedStream(size int64) *BoundedStream {
	return &BoundedStream{
		stream: make(chan []byte, size),
		size:   size,
	}
}

// Recv blocks until data is available or the stream is closed.
func (boundedStream *BoundedStream) Recv() ([]byte, bool) {
	data, ok := <-boundedStream.stream
	return data, ok
}

func (boundedStream *BoundedStream) Send(stream []byte) {
	boundedStream.stream <- stream
}

// Close stream to indicate the end of data.
func (boundedStream *BoundedStream) Close() {
	close(boundedStream.stream)
}
