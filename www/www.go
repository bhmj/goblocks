package www

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/bhmj/goblocks/file"
)

var (
	client *http.Client
)

type timeoutErrorType struct{}

func (t *timeoutErrorType) Error() string { return "timeout" }

const getTimeout = time.Duration(444 * time.Second) // FIXME

func init() {
	client = &http.Client{
		Timeout: getTimeout,
	}
}

func setHeaders(req *http.Request) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Accept-Language", "en-GB;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="118", "Google Chrome";v="118", "Not=A?Brand";v="99"`)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36")
}

type WWWInterface interface {
	// async download to file
	EnqueueDownload(url, root, path string) (extPath string, err error)
	// sync download to file
	Download(url, root, path, name string) error
	// sync
	DownloadContent(url, root, path string) (string, []byte, string, error)
	FetchContent(url, root, path string) ([]byte, string, error)
}

// SetContentType is a callback function which receives a contentType of a downloaded file once it has beed saved to disk
type SetContentType func(url, contentType string, fileSize int64)

// EnqueueDownload makes one attempt to download file synchronously and in case of timeout run the ansychronous
// process with a log scale retry policy. If the file was downloaded at first attempt, the contentType is not empty.
func EnqueueDownload(url, root, path string, sct SetContentType) (extPath string, contentType string, fileSize int64, err error) {
	extPath, fname, err := file.GenerateRandomFilename(url, root, path)
	if err != nil {
		return "", "", 0, err
	}

	contentType, fileSize, err = Download(url, root, path, fname)

	var timeoutErr *timeoutErrorType
	if errors.As(err, &timeoutErr) {
		go func() {
			retryPolicy := []int{2, 4, 8, 16, 32, 64, 128}
			for _, delay := range retryPolicy {
				time.Sleep(time.Duration(delay) * time.Second)
				contentType, fileSize, err = Download(url, root, path, fname)
				if !errors.As(err, &timeoutErr) {
					sct(url, contentType, fileSize)
					return
				}
			}
		}()
		return extPath, contentType, fileSize, nil // delayed download
	}
	return extPath, contentType, fileSize, err
}

// Download attempts to download a file into specified location. Returns contentType or error.
func Download(url, root, path, fname string, opts ...RequestOpt) (string, int64, error) {
	ct, _, fileSize, err := Fetch(url, root, path, fname, nil, opts...)
	return ct, fileSize, err
}

// DownloadContent attempts to download a file into specified location and returns the downloaded file external path
// and the body along with contentType.
func DownloadContent(url, root, path string) (string, []byte, string, int64, error) {
	extPath, fname, err := file.GenerateRandomFilename(url, root, path)
	if err != nil {
		return "", nil, "", 0, err
	}
	buf := &bytes.Buffer{}
	contentType, _, fileSize, err := Fetch(url, root, path, fname, buf)
	return extPath, buf.Bytes(), contentType, fileSize, err
}

// FetchContent attempts to download a file and return its content along with a new URL if redirect occured.
func FetchContent(url string, opts ...RequestOpt) ([]byte, string, *url.URL, int64, error) {
	buf := &bytes.Buffer{}
	contentType, newURL, fileSize, err := Fetch(url, "", "", "", buf, opts...)
	return buf.Bytes(), contentType, newURL, fileSize, err
}

// Fetch downloads a file specified in uri, saves it to root+path+fname (if fname specified), copies the body content into buf
// (if buf specified) and returns newURL if redirect occured.
func Fetch(url, root, path, fname string, buf io.Writer, opts ...RequestOpt) (contentType string, newURL *url.URL, fileSize int64, err error) {
	body, contentType, newURL, err := getResponse(url, opts...)
	if err != nil {
		return
	}
	defer body.Close()

	if fname != "" {
		// save to file; optionally read to buf
		fpath := filepath.Join(root, path)
		if !file.Exists(fpath) {
			if err = file.Mkdir(fpath); err != nil {
				return
			}
		}
		fullPath := filepath.Join(fpath, fname)
		var file *os.File
		file, err = os.Create(fullPath)
		if err != nil {
			return
		}
		defer file.Close()

		reader := body.(io.Reader)
		if buf != nil {
			reader = io.TeeReader(body, buf)
		}
		fileSize, err = io.Copy(file, reader)
	} else {
		// read to buf
		fileSize, err = io.Copy(buf, body)
	}

	return
}

type RequestOpt int

const (
	ReqIgnore403 RequestOpt = 1 // forbidden
	ReqIgnore404 RequestOpt = 2 // not found
	ReqIgnore406 RequestOpt = 3 // not acceptable
	ReqIgnore410 RequestOpt = 4 // gone
	ReqIgnore451 RequestOpt = 5 // unavailable for leagal reasons
)

type gzipReadCloser struct {
	body     io.ReadCloser
	gzreader io.Reader
}

func (z gzipReadCloser) Read(p []byte) (int, error) {
	return z.gzreader.Read(p)
}
func (z gzipReadCloser) Close() error {
	return z.body.Close()
}

// getResponse
func getResponse(uri string, opts ...RequestOpt) (io.ReadCloser, string, *url.URL, error) {
	opt := func(chk RequestOpt) bool {
		for _, val := range opts {
			if val == chk {
				return true
			}
		}
		return false
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, uri, nil)
	if err != nil {
		return nil, "", nil, err // nolint:wrapcheck
	}
	setHeaders(req)
	response, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, "", nil, &timeoutErrorType{}
		}
		return nil, "", nil, err // nolint:wrapcheck
	}

	body := response.Body
	if response.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(response.Body)
		if err != nil {
			response.Body.Close()
			return nil, "", nil, err
		}
		body = gzipReadCloser{
			body:     response.Body,
			gzreader: gr,
		}
	}

	var newURL *url.URL
	if response.Request != nil {
		newURL = response.Request.URL
	}

	switch {
	case response.StatusCode == http.StatusOK:
	case response.StatusCode == http.StatusForbidden && opt(ReqIgnore403):
	case response.StatusCode == http.StatusNotFound && opt(ReqIgnore404):
	case response.StatusCode == http.StatusNotAcceptable && opt(ReqIgnore406):
	case response.StatusCode == http.StatusGone && opt(ReqIgnore410):
	case response.StatusCode == http.StatusUnavailableForLegalReasons && opt(ReqIgnore451):
		break
	default:
		b, _ := io.ReadAll(response.Body)
		response.Body.Close()
		return nil, "", newURL, fmt.Errorf("received non 200 response code: %v; %s", response.StatusCode, string(b)) // nolint:goerr113
	}

	contentType := ""
	cts := response.Header["Content-Type"]
	if len(cts) > 0 {
		contentType = cts[0] // first occurrence
	}

	return body, contentType, newURL, nil
}
