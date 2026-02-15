package abstract

type DB interface {
	BeginTransaction() (DB, error)
	Rollback() error
	Commit() error
	Connect() error
	Query(dst any, query string, args ...any) error
	QueryRow(dst any, query string, args ...any) (bool, error)
	QueryValue(dst any, query string, args ...any) error
	Exec(query string, args ...any) error
	Close()
}
