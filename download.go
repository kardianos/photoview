package main

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/disintegration/imaging"
)

type baseList struct {
	folder string
	list   []string
	small  bool
}

var downloadMapper = make(map[string]baseList)

func (p *program) downloadAssignKey(folder string, list []string, small bool) (key string, err error) {
	kb := make([]byte, 32)
	_, err = rand.Read(kb)
	if err != nil {
		return "", err
	}
	key = base32.StdEncoding.EncodeToString(kb)
	downloadMapper[key] = baseList{folder, list, small}
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

	result := &bytes.Buffer{}

	for _, item := range bl.list {
		filename := filepath.Join(config.FileRoot, bl.folder, item)
		fi, err := os.Stat(filename)
		if err != nil {
			return err
		}
		if !bl.small {
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
		} else {
			fullImage, err := imaging.Open(filename)
			if err != nil {
				return err
			}
			resized := imaging.Fit(fullImage, config.SmallRes, config.SmallRes, imaging.Linear)

			err = imaging.Encode(result, resized, imaging.JPEG)
			if err != nil {
				return err
			}

			header := &tar.Header{
				Name:    item,
				Size:    int64(result.Len()),
				Mode:    0666,
				ModTime: fi.ModTime(),
			}
			err = tw.WriteHeader(header)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, result)
			if err != nil {
				return err
			}
			result.Reset()
		}
	}

	return tw.Close()
}

func getImage(filename string, small bool) (io.ReadCloser, error) {
	if !small {
		return os.Open(filename)
	}

	fullImage, err := imaging.Open(filename)
	if err != nil {
		return nil, err
	}
	resized := imaging.Fit(fullImage, config.SmallRes, config.SmallRes, imaging.Linear)

	r, w := io.Pipe()
	go imaging.Encode(w, resized, imaging.JPEG)
	return r, nil
}
