package main

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
)

var movieTimes = []string{"00:00:05", "00:00:01", "00:00:00"}

func (p *program) serveFile(w http.ResponseWriter, r *http.Request, fullPath, urlPath, resolutionString string) error {
	if len(resolutionString) == 0 {
		http.ServeFile(w, r, fullPath)
		return nil
	}
	createThumbFrom := fullPath
	imgSize, err := strconv.Atoi(resolutionString)
	if err != nil {
		return err
	}
	isMovie := false
	cachePath := filepath.Join(p.execDir, "cache", r.URL.Path)
	if strings.HasSuffix(urlPath, ".mp4") {
		// Movie: avconv -i /data/store/Pictures/2015-Q1/VID_20150125_1928.mp4 -vframes 1 -ss 00:00:01 out.jpg
		createThumbFrom = cachePath + ".orig.jpg"
		cachePath = cachePath + ".jpg"
		isMovie = true
	}
	_, err = os.Stat(cachePath)
	if os.IsNotExist(err) {
		if isMovie {
			var output []byte
			for _, time := range movieTimes {
				cmd := exec.Command("avconv", "-i", fullPath, "-vframes", "1", "-ss", time, createThumbFrom)
				output, err = cmd.CombinedOutput()
				if err == nil {
					_, err = os.Stat(createThumbFrom)
					// Make sure avconv wrote file.
					if os.IsNotExist(err) {
						continue
					}
					break
				}
			}
			if err != nil {
				logger.Errorf("Problem converting thumbnail for movie: %v\n%s\n", err, output)
				createThumbFrom = filepath.Join(p.execDir, "template", badThumb)
			}
		}
		// Resize image, open cache image.
		cacheDir, _ := filepath.Split(cachePath)
		err := os.MkdirAll(cacheDir, 0777)
		if err != nil {
			return err
		}
		fullImage, err := imaging.Open(createThumbFrom)
		if err != nil {
			return err
		}
		resized := imaging.Fit(fullImage, imgSize, imgSize, imaging.Linear)
		// Um, assume JPG for now.
		cf, err := os.Create(cachePath)
		if err != nil {
			return err
		}
		err = imaging.Encode(cf, resized, imaging.JPEG)
		cf.Close()
		if err != nil {
			return err
		}
	}
	http.ServeFile(w, r, cachePath)
	return nil
}
