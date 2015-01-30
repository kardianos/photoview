package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
)

type Rot byte

const (
	RotLeft Rot = iota
	RotRight
	RotFlip
)

func (p *program) editImage(rot Rot, folder string, list []string) error {
	for _, item := range list {
		filename := filepath.Join(FileRoot, folder, item)
		fi, err := os.Stat(filename)
		if err != nil {
			return err
		}
		fullImage, err := imaging.Open(filename)
		if err != nil {
			return err
		}

		switch rot {
		case RotLeft:
			fullImage = imaging.Rotate90(fullImage)
		case RotRight:
			fullImage = imaging.Rotate270(fullImage)
		case RotFlip:
			fullImage = imaging.Rotate180(fullImage)
		default:
			return fmt.Errorf("Unknown rot value: %d", rot)
		}
		err = imaging.Save(fullImage, filename)
		if err != nil {
			return err
		}
		tm := fi.ModTime()
		err = os.Chtimes(filename, tm, tm)
		if err != nil {
			return err
		}
	}

	cacheFolder := filepath.Join(p.execDir, "cache", folder)
	cf, err := os.Open(cacheFolder)
	if err != nil {
		return err
	}
	defer cf.Close()

	// Remove cache items.
	fileList, err := cf.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, file := range fileList {
		for _, item := range list {
			if strings.HasPrefix(file, item) {
				err = os.Remove(filepath.Join(cacheFolder, file))
				if err != nil {
					logger.Error("Failed to remove cached file ", file, ": ", err)
				}
				break
			}
		}
	}
	return nil
}
