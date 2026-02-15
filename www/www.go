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
	"slices"
	"time"

	"github.com/bhmj/goblocks/file"
)

var client *http.Client //nolint:gochecknoglobals

type timeoutError struct{}

func (t *timeoutError) Error() string { return "timeout" }

const getTimeout = 444 * time.Second // FIXME

func init() { //nolint:gochecknoinits
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

type Interface interface {
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

	var timeoutErr *timeoutError
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

// FetchContent attempts to download a file and return its content along with a new URL if redirect occurred.
func FetchContent(url string, opts ...RequestOpt) ([]byte, string, *url.URL, int64, error) {
	buf := &bytes.Buffer{}
	contentType, newURL, fileSize, err := Fetch(url, "", "", "", buf, opts...)
	return buf.Bytes(), contentType, newURL, fileSize, err
}

// Fetch downloads a file specified in uri, saves it to root+path+fname (if fname specified), copies the body content into buf
// (if buf specified) and returns newURL if redirect occurred.
func Fetch(url, root, path, fname string, buf io.Writer, opts ...RequestOpt) (contentType string, newURL *url.URL, fileSize int64, err error) {
	body, contentType, newURL, err := getResponse(url, opts...)
	if err != nil {
		return //nolint:nakedret
	}
	defer body.Close()

	if fname == "" {
		fileSize, err = io.Copy(buf, body)
		return //nolint:nakedret
	}

	// save to file; optionally read to buf
	fpath := filepath.Join(root, path)
	if !file.Exists(fpath) {
		if err = file.Mkdir(fpath); err != nil {
			return //nolint:nakedret
		}
	}
	fullPath := filepath.Join(fpath, fname)
	var file *os.File
	file, err = os.Create(fullPath)
	if err != nil {
		return //nolint:nakedret
	}
	defer file.Close()

	reader := body.(io.Reader) //nolint:forcetypeassert
	if buf != nil {
		reader = io.TeeReader(body, buf)
	}
	fileSize, err = io.Copy(file, reader)

	return //nolint:nakedret
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

func hasOpt(opts []RequestOpt, chk RequestOpt) bool {
	return slices.Contains(opts, chk)
}

// ignorableStatus maps HTTP status codes to the RequestOpt that allows ignoring them.
var ignorableStatus = map[int]RequestOpt{ //nolint:gochecknoglobals
	http.StatusForbidden:                  ReqIgnore403,
	http.StatusNotFound:                   ReqIgnore404,
	http.StatusNotAcceptable:              ReqIgnore406,
	http.StatusGone:                       ReqIgnore410,
	http.StatusUnavailableForLegalReasons: ReqIgnore451,
}

func checkStatusCode(response *http.Response, opts []RequestOpt) error {
	if response.StatusCode == http.StatusOK {
		return nil
	}
	if opt, ok := ignorableStatus[response.StatusCode]; ok && hasOpt(opts, opt) {
		return nil
	}
	b, _ := io.ReadAll(response.Body)
	response.Body.Close()
	return fmt.Errorf("received non 200 response code: %v; %s", response.StatusCode, string(b))
}

func wrapGzipBody(response *http.Response) (io.ReadCloser, error) {
	if response.Header.Get("Content-Encoding") != "gzip" {
		return response.Body, nil
	}
	gr, err := gzip.NewReader(response.Body)
	if err != nil {
		response.Body.Close()
		return nil, err
	}
	return gzipReadCloser{body: response.Body, gzreader: gr}, nil
}

func redirectURL(response *http.Response) *url.URL {
	if response.Request != nil {
		return response.Request.URL
	}
	return nil
}

func doRequest(uri string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	setHeaders(req)
	response, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, &timeoutError{}
		}
		return nil, err
	}
	return response, nil
}

// getResponse
func getResponse(uri string, opts ...RequestOpt) (io.ReadCloser, string, *url.URL, error) {
	response, err := doRequest(uri)
	if err != nil {
		return nil, "", nil, err
	}

	newURL := redirectURL(response)

	if err := checkStatusCode(response, opts); err != nil {
		return nil, "", newURL, err
	}

	body, err := wrapGzipBody(response)
	if err != nil {
		return nil, "", nil, err
	}

	contentType := response.Header.Get("Content-Type")

	return body, contentType, newURL, nil
}
