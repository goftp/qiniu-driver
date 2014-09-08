package qiniudriver

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/goftp/server"
	"github.com/qiniu/api/auth/digest"
	"github.com/qiniu/api/conf"
	qio "github.com/qiniu/api/io"
	"github.com/qiniu/api/rs"
	"github.com/qiniu/api/rsf"
)

type QiniuDriver struct {
	curDir  string
	client  rs.Client
	client2 rsf.Client
	bucket  string
}

func (driver *QiniuDriver) ChangeDir(path string) error {
	driver.curDir = path
	return nil
}

func (driver *QiniuDriver) Stat(key string) (server.FileInfo, error) {
	entry, err := driver.client.Stat(nil, driver.bucket, key)
	if err != nil {
		return nil, err
	}

	return &FileInfo{key, entry}, nil
}

func (driver *QiniuDriver) DirContents(prefix string) ([]server.FileInfo, error) {
	fmt.Print("bucket:", driver.bucket, "prefix:", strings.TrimLeft(prefix, "/"))
	entries, _, err := driver.client2.ListPrefix(nil, driver.bucket, strings.TrimLeft(prefix, "/"), "", 1000)
	if err != nil {
		return nil, err
	}

	files := make([]server.FileInfo, 0, len(entries))
	for _, entry := range entries {
		files = append(files, &FileInfo{
			name: entry.Key,
			Entry: rs.Entry{
				Hash:     entry.Hash,
				Fsize:    entry.Fsize,
				PutTime:  entry.PutTime,
				MimeType: entry.MimeType,
				Customer: "",
			},
		})
	}

	return files, nil
}

func (driver *QiniuDriver) DeleteDir(key string) error {
	return driver.client.Delete(nil, driver.bucket, key)
}

func (driver *QiniuDriver) DeleteFile(key string) error {
	return driver.client.Delete(nil, driver.bucket, key)
}

func (driver *QiniuDriver) Rename(keySrc, keyDest string) error {
	return driver.client.Move(nil, driver.bucket,
		keySrc, driver.bucket, keyDest)
}

func (driver *QiniuDriver) MakeDir(path string) error {
	return nil
}

func (driver *QiniuDriver) GetFile(key string, start int64) (int64, io.ReadCloser, error) {
	stat, err := driver.Stat(key)
	if err != nil {
		return 0, nil, err
	}

	domain := fmt.Sprintf("%s.qiniudn.com", driver.bucket)
	baseUrl := rs.MakeBaseUrl(domain, key)
	policy := rs.GetPolicy{}
	downUrl := policy.MakeRequest(baseUrl, nil)

	resp, err := http.Get(downUrl)
	if err != nil {
		return 0, nil, err
	}

	return stat.Size(), resp.Body, nil
}

func (driver *QiniuDriver) PutFile(key string, data io.Reader, appendData bool) (int64, error) {
	var err error
	var ret qio.PutRet
	var extra = &qio.PutExtra{}

	putPolicy := rs.PutPolicy{
		Scope: driver.bucket,
	}
	uptoken := putPolicy.Token(nil)

	rd := CountReader(data)
	err = qio.Put(nil, &ret, uptoken, key, rd, extra)
	if err != nil {
		return 0, err
	}

	return int64(rd.Size()), nil
}

type QiniuDriverFactory struct {
	bucket string
}

func NewQiniuDriverFactory(accessKey, secretKey, bucket string) server.DriverFactory {
	conf.ACCESS_KEY = accessKey
	conf.SECRET_KEY = secretKey
	return &QiniuDriverFactory{bucket}
}

func (factory *QiniuDriverFactory) NewDriver() (server.Driver, error) {
	mac := &digest.Mac{conf.ACCESS_KEY, []byte(conf.SECRET_KEY)}
	client := rs.New(mac)
	client2 := rsf.New(mac)
	return &QiniuDriver{"/", client, client2, factory.bucket}, nil
}
