package qiniudriver

import (
	"os"
	"time"

	"github.com/qiniu/api.v6/rs"
)

type FileInfo struct {
	name  string
	isDir bool
	rs.Entry
}

func (f *FileInfo) Name() string {
	return f.name
}

func (f *FileInfo) Size() int64 {
	return f.Entry.Fsize
}

func (f *FileInfo) Mode() os.FileMode {
	if f.isDir {
		return os.ModeDir | os.ModePerm
	}
	return os.ModePerm
}

func (f *FileInfo) ModTime() time.Time {
	return time.Unix(0, f.Entry.PutTime*100)
}

func (f *FileInfo) IsDir() bool {
	return f.isDir
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
