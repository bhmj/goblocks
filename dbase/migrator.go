package dbase

import (
	"crypto/sha1" //nolint:gosec
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bhmj/goblocks/dbase/abstract"
	"github.com/bhmj/goblocks/file"
	"github.com/bhmj/goblocks/log"
)

var errMigrationDirNotFound = errors.New("migration dir not found")

type Migrator struct {
	logger log.MetaLogger
	db     abstract.DB
}

func NewMigrator(db abstract.DB, logger log.MetaLogger) *Migrator {
	return &Migrator{db: db, logger: logger}
}

func (m *Migrator) Migrate(basePath string) error {
	var err error

	if basePath == "" {
		return nil
	}

	if err = m.assureMigrationSupported(); err != nil {
		return err
	}

	basePath, err = file.NormalizePath(basePath)
	if err != nil {
		return err
	}

	if _, err = os.Stat(basePath); os.IsNotExist(err) {
		m.logger.Warn("migrator", log.Error(errMigrationDirNotFound), log.String("normalized path", basePath))
		return errMigrationDirNotFound
	}

	m.logger.Info("applying migrations...")
	objects := []string{"Schemas", "Tables", "Procedures", "Triggers", "Migrations"}
	for i := range objects {
		if err := m.processFilesIn(basePath, objects[i]); err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) assureMigrationSupported() error {
	var result bool
	sql := `select exists (
		select from information_schema.tables 
		where  table_schema = 'public'
		and    table_name   = 'schema_migrations'
	);`
	err := m.db.QueryValue(&result, sql)
	if err != nil {
		return err //nolint:wrapcheck
	}
	if !result {
		sql = `create table public.schema_migrations (
			id serial4 not null,
			object_name text,
			hash bytea,
			dt timestamp default now(),
			constraint pk_schema_migrations primary key (id),
			unique(object_name)
		)`
		err = m.db.Exec(sql)
		if err != nil {
			return err //nolint:wrapcheck
		}
	}
	return nil
}

func (m *Migrator) processFilesIn(basePath, inPath string) error {
	var err error

	fullPath := filepath.Join(basePath, inPath)
	if _, err = os.Stat(fullPath); os.IsNotExist(err) {
		m.logger.Warn("migration dir not found", log.String("path", fullPath))
		return nil
	}

	files, _ := os.ReadDir(fullPath)
	// sort files by name
	sort.Slice(files, func(i, j int) bool {
		return strings.Compare(files[i].Name(), files[j].Name()) < 0
	})
	// first directories
	for _, file := range files {
		if file.IsDir() {
			if err = m.processFilesIn(basePath, filepath.Join(inPath, file.Name())); err != nil {
				return err
			}
		}
	}
	// then files
	for _, file := range files {
		if !file.IsDir() {
			if err = m.applyMigration(basePath, filepath.Join(inPath, file.Name())); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Migrator) applyMigration(basePath, file string) error {
	fullPath := filepath.Join(basePath, file)
	// read file contents
	contents, err := m.readFileContents(fullPath)
	if err != nil {
		return err
	}
	// calc hash
	sha1 := sha1.Sum(contents) //nolint:gosec

	// find file record in schema_migrations
	var found bool
	sql := `select exists (select from public.schema_migrations where hash = $1)`
	err = m.db.QueryValue(&found, sql, sha1[:])
	if err != nil {
		return err //nolint:wrapcheck
	}
	if !found {
		m.logger.Info("migrator", log.String("new hash", hex.EncodeToString(sha1[:])))
		tx, err := m.db.BeginTransaction()
		if err != nil {
			m.logger.Error("migrator", log.String("db", "transaction"), log.Error(err))
			return err //nolint:wrapcheck
		}
		defer func() { _ = tx.Rollback() }()

		// apply migration
		err = tx.Exec(string(contents))
		if err != nil {
			m.logger.Error("failed", log.String("error", err.Error()), log.String("file", file))
			return err //nolint:wrapcheck
		}
		// store hash
		sql := `
			insert into public.schema_migrations (object_name, hash)
			values ($1, $2)
			on conflict (object_name) do update set
				hash = excluded.hash`
		err = tx.Exec(sql, filepath.Base(file), sha1[:])
		if err != nil {
			return err //nolint:wrapcheck
		}
		m.logger.Info("applied file", log.String("file", file))

		_ = tx.Commit()
	}
	return nil
}

func (m *Migrator) readFileContents(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}
	return b, nil
}
