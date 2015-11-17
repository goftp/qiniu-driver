package qiniudriver

import (
	"errors"
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
	f, err := driver.Stat(path)
	if err != nil {
		return err
	}

	if !f.IsDir() {
		return errors.New("not a dir")
	}
	driver.curDir = path
	return nil
}

func (driver *QiniuDriver) Stat(key string) (server.FileInfo, error) {
	if strings.HasSuffix(key, "/") {
		return &FileInfo{key, true, rs.Entry{}}, nil
	}
	entry, err := driver.client.Stat(nil, driver.bucket, strings.TrimLeft(key, "/"))
	if err != nil {
		entries, _, _ := driver.client2.ListPrefix(nil, driver.bucket, strings.TrimLeft(key, "/")+"/", "", 1)
		if len(entries) > 0 {
			return &FileInfo{key, true, rs.Entry{}}, nil
		}
		return nil, errors.New("dir not exists")
	}

	return &FileInfo{key, false, entry}, nil
}

func (driver *QiniuDriver) ListDir(prefix string, callback func(server.FileInfo) error) error {
	d := strings.TrimLeft(prefix, "/")
	if d != "" {
		d = d + "/"
	}
	entries, _, err := driver.client2.ListPrefix(nil, driver.bucket, d, "", 1000)
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		return err
	}

	dirCache := make(map[string]bool)

	for _, entry := range entries {
		if prefix != "/" && prefix != "" && !strings.HasPrefix(entry.Key, d) {
			continue
		}
		key := strings.TrimLeft(strings.TrimLeft(entry.Key, d), "/")
		if key == "" {
			continue
		}
		var f server.FileInfo
		if strings.Contains(key, "/") {
			key := strings.Trim(strings.Split(key, "/")[0], "/")
			if _, ok := dirCache[key]; ok {
				continue
			}
			dirCache[key] = true
			f = &FileInfo{name: key, isDir: true}
		} else {
			f = &FileInfo{
				name: key,
				Entry: rs.Entry{
					Hash:     entry.Hash,
					Fsize:    entry.Fsize,
					PutTime:  entry.PutTime,
					MimeType: entry.MimeType,
					Customer: "",
				},
			}
		}
		err = callback(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func (driver *QiniuDriver) DeleteDir(key string) error {
	d := strings.TrimLeft(key, "/")

	entries, _, err := driver.client2.ListPrefix(nil, driver.bucket, d, "", 1000)
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	delentries := make([]rs.EntryPath, 0)
	for _, entry := range entries {
		delentries = append(delentries, rs.EntryPath{driver.bucket, entry.Key})
	}

	fmt.Println("delete entries:", delentries)

	_, err = driver.client.BatchDelete(nil, delentries)
	return err
}

func (driver *QiniuDriver) DeleteFile(key string) error {
	fmt.Println("delete file", key)
	return driver.client.Delete(nil, driver.bucket, strings.TrimLeft(key, "/"))
}

func (driver *QiniuDriver) Rename(keySrc, keyDest string) error {
	fmt.Println("rename from", keySrc, keyDest)
	var from = strings.TrimLeft(keySrc, "/")
	var to = strings.TrimLeft(keyDest, "/")
	info, err := driver.client.Stat(nil, driver.bucket, from)
	if err != nil && strings.Contains(err.Error(), "no such file or directory") {
		from = strings.TrimLeft(keySrc, "/") + "/"
		to = strings.TrimLeft(keyDest, "/") + "/"
		info, err = driver.client.Stat(nil, driver.bucket, from)
		if err != nil {
			return err
		}
		entries, _, err := driver.client2.ListPrefix(nil, driver.bucket, from, "", 1000)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			newKey := strings.Replace(entry.Key, from, to, 1)
			err = driver.client.Move(nil, driver.bucket, entry.Key, driver.bucket, newKey)
			if err != nil {
				return err
			}
		}
		return nil
	}
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println(info, from, to)
	return driver.client.Move(nil, driver.bucket, from, driver.bucket, to)
}

func (driver *QiniuDriver) MakeDir(path string) error {
	dir := strings.TrimLeft(path, "/") + "/"
	fmt.Println("mkdir", dir)
	var s string
	reader := strings.NewReader(s)
	_, err := driver.PutFile(dir, reader, false)
	return err
}

func (driver *QiniuDriver) GetFile(key string, start int64) (int64, io.ReadCloser, error) {
	stat, err := driver.Stat(key)
	if err != nil {
		return 0, nil, err
	}

	key = strings.TrimLeft(key, "/")

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
	err = qio.Put(nil, &ret, uptoken, strings.TrimLeft(key, "/"), rd, extra)
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
