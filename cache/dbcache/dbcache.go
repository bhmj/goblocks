package dbcache

import (
	"path/filepath"
	"time"

	"github.com/bhmj/goblocks/dbase/abstract"
	"github.com/bhmj/goblocks/file"
	"github.com/bhmj/goblocks/log"
	"github.com/bhmj/goblocks/www"
)

type cache struct {
	db       abstract.DB
	logger   log.MetaLogger
	cacheDir string
}

type Cache interface {
	GetURL(url string) (string, error)
	GetContent(url string) ([]byte, string, error)
	Cleanup()
}

func New(db abstract.DB, logger log.MetaLogger, cacheDir string) Cache {
	return &cache{
		db:       db,
		logger:   logger,
		cacheDir: cacheDir,
	}
}

func (c *cache) GetURL(url string) (extPath string, err error) {
	extPath, _ = c.getCacheRecord(url)
	fullPath := filepath.Join(c.cacheDir, extPath)
	if extPath != "" && file.Exists(fullPath) {
		return
	}
	extPath, contentType, fileSize, err := c.requestURL(url)
	if err != nil {
		return
	}

	err = c.setCacheRecord(url, extPath, contentType, fileSize)
	return
}

func (c *cache) GetContent(url string) (body []byte, contentType string, err error) {
	extPath, contentType := c.getCacheRecord(url)
	fullPath := filepath.Join(c.cacheDir, extPath)
	if extPath != "" && file.Exists(fullPath) {
		body, err = file.Read(fullPath)
		return
	}
	extPath, body, contentType, fileSize, err := c.fetchURL(url)
	if err != nil {
		return
	}
	err = c.setCacheRecord(url, extPath, contentType, fileSize)
	return
}

type cacheRec struct {
	FilePath    string    `db:"file_path"`
	ContentType string    `db:"content_type"`
	AddedAt     time.Time `db:"added_at"`
}

func (c *cache) getCacheRecord(url string) (string, string) {
	// TODO: add memory cache
	var entry cacheRec
	sql := `
		with upd as (
			update file_cache set
			  last_read_at = now()
			where source_url = $1
			returning id
		)
	  select file_path, content_type, added_at
		from file_cache
		where id = (select id from upd limit 1)`
	found, err := c.db.QueryRow(&entry, sql, url)
	if err != nil {
		c.logger.Error("getting cache record", log.Error(err))
		return "", ""
	}
	if !found {
		c.logger.Info("getting cache record: not found", log.String("url", url))
	}
	return entry.FilePath, entry.ContentType
}

func (c *cache) setCacheRecord(url, extPath, contentType string, fileSize int64) error {
	// TODO: update memory cache
	sql := `
		insert into file_cache (
			source_url, file_path, content_type, file_size
		) values (
			$1, $2, $3, $4
		) on conflict (source_url) do update set
			file_path = excluded.file_path,
			content_type = excluded.content_type
		;`
	return c.db.Exec(sql, url, extPath, contentType, fileSize)
}

func (c *cache) contentTypeUpdate(url, contentType string, fileSize int64) {
	sql := `update file_cache set content_type = $1, file_size = $2 where source_url = $3`
	err := c.db.Exec(sql, contentType, fileSize, url)
	if err != nil {
		c.logger.Error("updating content_type", log.Error(err))
	}
}

func (c *cache) requestURL(url string) (string, string, int64, error) {
	path := time.Now().Format("2006-01-02")
	return www.EnqueueDownload(url, c.cacheDir, path, c.contentTypeUpdate)
}

func (c *cache) fetchURL(url string) (string, []byte, string, int64, error) {
	path := time.Now().Format("2006-01-02")
	return www.DownloadContent(url, c.cacheDir, path)
}

func (c *cache) Cleanup() {}
