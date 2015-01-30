package main

import (
	"os"
	"sort"
	"strings"
)

var tmplFunc = map[string]interface{}{
	"isImage": func(name string) bool {
		isImage := false
		isImage = isImage || strings.HasSuffix(name, ".jpg")
		isImage = isImage || strings.HasSuffix(name, ".jpeg")
		isImage = isImage || strings.HasSuffix(name, ".png")
		return isImage
	},
	"isMovie": func(name string) bool {
		isImage := false
		isImage = isImage || strings.HasSuffix(name, ".mp4")
		return isImage
	},
}

type sortFileInfoModTime []os.FileInfo

func (s sortFileInfoModTime) Len() int           { return len(s) }
func (s sortFileInfoModTime) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortFileInfoModTime) Less(i, j int) bool { return s[i].ModTime().Before(s[j].ModTime()) }

type sortFileInfoName []os.FileInfo

func (s sortFileInfoName) Len() int           { return len(s) }
func (s sortFileInfoName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortFileInfoName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }

type DirTmpl struct {
	Res       uint
	UrlParts  []string
	ItemsName []os.FileInfo
	ItemsTime []os.FileInfo
}

func NewDirTmpl(fullPath, urlPath string) (*DirTmpl, error) {
	dt := &DirTmpl{
		Res: ThumbRes,
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ii, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}
	dt.ItemsName = ii
	dt.ItemsTime = make([]os.FileInfo, len(ii))
	copy(dt.ItemsTime, ii)

	sort.Sort(sortFileInfoName(dt.ItemsName))
	sort.Sort(sortFileInfoModTime(dt.ItemsTime))

	dt.UrlParts = strings.Split(urlPath[1:], "/")

	return dt, nil
}
