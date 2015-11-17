package qiniudriver

import (
	"io"
	"io/ioutil"
)

func NewSkipReadCloser(rd io.ReadCloser, count int64) io.ReadCloser {
	return &SkipReadCloser{
		ReadCloser: rd,
		count:      count,
	}
}

type SkipReadCloser struct {
	io.ReadCloser
	count   int64
	skipped bool
}

func (s *SkipReadCloser) Read(data []byte) (int, error) {
	if !s.skipped {
		if s.count > 0 {
			_, err := io.CopyN(ioutil.Discard, s.ReadCloser, s.count)
			if err != nil {
				return 0, err
			}
		}
		s.skipped = true
	}
	return s.ReadCloser.Read(data)
}
