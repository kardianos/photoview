package main

import (
	"archive/tar"
	"crypto/rand"
	"encoding/base32"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type baseList struct {
	folder string
	list   []string
}

var downloadMapper = make(map[string]baseList)

func (p *program) downloadAssignKey(folder string, list []string) (key string, err error) {
	kb := make([]byte, 32)
	_, err = rand.Read(kb)
	if err != nil {
		return "", err
	}
	key = base32.StdEncoding.EncodeToString(kb)
	downloadMapper[key] = baseList{folder, list}
	go func() {
		<-time.After(time.Second * 3)
		delete(downloadMapper, key)
	}()

	return key, nil
}

func (p *program) downloadWriteKey(key string, w http.ResponseWriter) error {
	bl, found := downloadMapper[key]
	if !found {
		http.Error(w, "Not found", 404)
		return nil
	}

	w.Header().Set("Content-Disposition", `attachment; filename="photos.tar"`)

	tw := tar.NewWriter(w)

	for _, item := range bl.list {
		filename := filepath.Join(FileRoot, bl.folder, item)
		fi, err := os.Stat(filename)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		err = tw.WriteHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, f)
		f.Close()
		if err != nil {
			return err
		}
	}

	return tw.Close()
}
