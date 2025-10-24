package dbase

import (
	"context"
	"regexp"
	"time"

	"github.com/bhmj/goblocks/dbase/abstract"
	"github.com/bhmj/goblocks/dbase/postgresql"
	"github.com/bhmj/goblocks/log"
)

type Config struct {
	Type       string `yaml:"type" description:"DB type" default:"postgres" choice:"postgres,mysql,sqlite,oracle,sqlserver"`
	ConnString string `yaml:"conn_string" description:"DB connection string" required:"true"`
	Migrations string `yaml:"migrations" description:"DB migrations path"`
}

const SkipMigration int = 1

func New(ctx context.Context, logger log.MetaLogger, cfg Config, options ...int) abstract.DB {
	var err error

	var db abstract.DB

	if cfg.Type != "postgres" {
		logger.Error("unsupported DB type", log.String("type", cfg.Type))
		return nil
	}

	// get DB name from connection string
	reDBName := regexp.MustCompile(`dbname=(\w+)`)
	res := reDBName.FindStringSubmatch(cfg.ConnString)
	dbName := "?"
	if res != nil {
		dbName = res[1]
	}

	// dumb
	delay := time.Second
	retries := 10
	for i := range retries {
		db, err = postgresql.New(ctx, cfg.ConnString) // establishes one connection!
		if err != nil {
			logger.Error("postgresql.New", log.Error(err), log.String("dbname", dbName))
			if i < retries-1 {
				time.Sleep(delay)
				delay = (delay * 120) / 100 //nolint:mnd
			}
		} else {
			break
		}
	}
	if err != nil {
		return nil
	}

	logger.Info("connecting to database", log.String("name", dbName))
	if err = db.Connect(); err != nil {
		logger.Error("DB.connect", log.Error(err), log.String("dbname", dbName))
		return nil
	}

	go func() { <-ctx.Done(); db.Close() }()

	skip := false
	for _, op := range options {
		if op == SkipMigration {
			skip = true
		}
	}
	if !skip {
		migrator := NewMigrator(db, logger)
		if err = migrator.Migrate(cfg.Migrations); err != nil {
			logger.Error("migration", log.Error(err), log.String("dbname", dbName))
			return nil
		}
	}

	return db
}
