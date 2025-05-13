package abstract

type DB interface {
	BeginTransaction() (DB, error)
	Rollback() error
	Commit() error
	Connect() error
	Query(dst interface{}, query string, args ...interface{}) error
	QueryRow(dst interface{}, query string, args ...interface{}) (bool, error)
	QueryValue(dst interface{}, query string, args ...interface{}) error
	Exec(query string, args ...interface{}) error
	Close()
}
