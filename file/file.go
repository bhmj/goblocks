package file

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bhmj/goblocks/str"
)

type FileInterface interface {
	Exists(fname string) bool
	Copy(src, dest string) (int64, error)
	Delete(fname string) error
	Mkdir(path string) error
	Rmdir(path string) error
	Move(src, dst string) error
	URLFileExtension(addr string) string
	Read(fname string) (contents []byte, err error)
	TouchWithPath(fname string, template string) error
	ClearDirectory(path string) error
}

func Exists(fname string) bool {
	if fname == "" {
		return false
	}
	if _, err := os.Stat(fname); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func Copy(src, dest string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err // nolint:wrapcheck
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err // nolint:wrapcheck
	}
	defer source.Close()

	dir := filepath.Dir(dest)
	if err := Mkdir(dir); err != nil {
		return 0, err // nolint:wrapcheck
	}

	destination, err := os.Create(dest)
	if err != nil {
		return 0, err // nolint:wrapcheck
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err // nolint:wrapcheck
}

func Delete(fname string) error {
	if _, err := os.Stat(fname); errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Remove(fname)
}

func Mkdir(path string) error {
	return os.MkdirAll(path, os.ModePerm) // nolint:wrapcheck
}

func Rmdir(path string) error {
	return os.RemoveAll(path)
}

func Move(src, dst string) error {
	return os.Rename(src, dst)
}

func URLFileExtension(addr string) string {
	u, err := url.Parse(addr)
	if err != nil {
		return ""
	}
	return filepath.Ext(u.Path)
}

func GenerateRandomFilename(url, root, path string) (string, string, error) {
	var fname string
	var fullName string
	ext := URLFileExtension(url)
	for {
		fname = strings.ReplaceAll(time.Now().Format("15-04-05.000"), ".", "-") + "-" + str.RandomString(4) // nolint:gomnd
		fullName = filepath.Join(root, path, fname+ext)
		if !Exists(fullName) {
			break
		}
	}
	return filepath.Join(path, fname+ext), fname + ext, nil // external filename (web)
}

func Read(fname string) (contents []byte, err error) {
	file, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}

// TouchWithPath ensures that file "fname" exists. If file does not exist, it is created as a copy of the
// specified template, including all the necessary parent directories.
func TouchWithPath(fname string, template string) error {
	dir := filepath.Dir(fname)
	if err := Mkdir(dir); err != nil {
		return err
	}
	if Exists(fname) {
		return nil
	}
	Copy(template, fname)
	return nil
}

func ClearDirectory(path string, flat bool) error {
	// Read all files and subdirectories in the directory
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	// Iterate over each entry and remove it
	for _, entry := range entries {
		if flat && entry.IsDir() {
			continue
		}
		entryPath := path + "/" + entry.Name()
		if err := os.RemoveAll(entryPath); err != nil {
			return err
		}
	}

	return nil
}
