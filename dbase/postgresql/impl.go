package postgresql

import (
	"context"
	"errors"

	"github.com/bhmj/goblocks/dbase/abstract"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool" // PostgreSQL driver
)

var (
	errNoTransactionOnRollback = errors.New("no transaction on rollback")
	errNoTransactionOnCommit   = errors.New("no transaction on commit")
)

type pgxQuerier interface {
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
}

type Psql struct {
	ctx  context.Context //nolint:containedctx
	pool *pgxpool.Pool   // connection pool
	conn pgxQuerier      // active connection
	tx   pgx.Tx          // current transaction
}

func New(ctx context.Context, conn string) (abstract.DB, error) {
	config, err := pgxpool.ParseConfig(conn)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	pool, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	return &Psql{
		ctx:  ctx,
		pool: pool,
		conn: pool,
	}, nil
}

func (p *Psql) BeginTransaction() (abstract.DB, error) {
	var tx pgx.Tx
	var err error
	if p.tx != nil {
		tx, err = p.tx.Begin(context.Background())
	} else {
		tx, err = p.pool.BeginTx(context.Background(), pgx.TxOptions{})
	}
	if err != nil {
		return nil, err //nolint:wrapcheck
	}
	return &Psql{ctx: p.ctx, pool: p.pool, conn: tx, tx: tx}, nil
}

func (p *Psql) Rollback() error {
	if p.tx == nil {
		return errNoTransactionOnRollback
	}
	err := p.tx.Rollback(context.Background())
	p.tx = nil
	p.conn = p.pool
	return err
}

func (p *Psql) Commit() error {
	if p.tx == nil {
		return errNoTransactionOnCommit
	}
	err := p.tx.Commit(context.Background())
	p.tx = nil
	p.conn = p.pool
	return err
}

func (p *Psql) Connect() error {
	return p.pool.Ping(p.ctx) //nolint:wrapcheck
}

func (p *Psql) Query(dst interface{}, query string, args ...interface{}) error {
	if len(args) == 0 {
		return pgxscan.Select(p.ctx, p.conn, dst, query) //nolint:wrapcheck
	}
	return pgxscan.Select(p.ctx, p.conn, dst, query, args...) //nolint:wrapcheck
}

func (p *Psql) QueryRow(dst interface{}, query string, args ...interface{}) (bool, error) {
	var err error
	if len(args) == 0 {
		err = pgxscan.Get(p.ctx, p.conn, dst, query)
	} else {
		err = pgxscan.Get(p.ctx, p.conn, dst, query, args...)
	}
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err //nolint:wrapcheck
}

func (p *Psql) QueryValue(dst interface{}, query string, args ...interface{}) error {
	row := p.conn.QueryRow(p.ctx, query, args...)
	return row.Scan(dst) //nolint:wrapcheck
}

func (p *Psql) Exec(query string, args ...interface{}) error {
	if len(args) == 0 {
		_, err := p.conn.Exec(p.ctx, query)
		return err //nolint:wrapcheck
	}
	_, err := p.conn.Exec(p.ctx, query, args...)
	return err //nolint:wrapcheck
}

func (p *Psql) Close() {
	if p.pool != nil {
		p.pool.Close()
	}
}
