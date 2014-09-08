package qiniudriver

import (
	"os"
	"time"

	"github.com/qiniu/api/rs"
)

type FileInfo struct {
	name string
	rs.Entry
}

func (f *FileInfo) Name() string {
	return f.name
}

func (f *FileInfo) Size() int64 {
	return f.Entry.Fsize
}

func (f *FileInfo) Mode() os.FileMode {
	return os.ModePerm
}

func (f *FileInfo) ModTime() time.Time {
	return time.Unix(0, f.Entry.PutTime*100)
}

func (f *FileInfo) IsDir() bool {
	return false
}

func (f *FileInfo) Sys() interface{} {
	return nil
}

func (f *FileInfo) Owner() string {
	return "qiniu"
}

func (f *FileInfo) Group() string {
	return "qiniu"
}
