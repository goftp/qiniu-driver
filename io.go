package qiniudriver

import "io"

type countReader struct {
	reader io.Reader
	counts int
}

func (cr *countReader) Size() int {
	return cr.counts
}

func (cr *countReader) Read(p []byte) (n int, err error) {
	rs, err := cr.reader.Read(p)
	cr.counts += rs
	return rs, err
}

// CountReader returns a Reader that's is just for counting the total bytes of read.
func CountReader(reader io.Reader) *countReader {
	return &countReader{reader, 0}
}
