package migrate

import (
	"github.com/ladzaretti/migrate/types"
)

// SQLiteDialect provides the needed queries for managing schema versioning
// for an SQLite database.
type SQLiteDialect struct{}

var _ types.Dialect = SQLiteDialect{}

func (SQLiteDialect) CreateVersionTableQuery() string {
	return `
		CREATE TABLE
			IF NOT EXISTS schema_version (
				id INTEGER PRIMARY KEY CHECK (id = 0),
				version INTEGER,
				checksum TEXT NOT NULL
			);
		`
}

func (SQLiteDialect) CurrentVersionQuery() string {
	return `SELECT id, version, checksum FROM schema_version;`
}

func (SQLiteDialect) SaveVersionQuery() string {
	return `
        	INSERT INTO schema_version (id, version, checksum)
        	VALUES (0, $1, $2)
        	ON CONFLICT(id) 
        	DO UPDATE SET version = EXCLUDED.version, checksum = EXCLUDED.checksum;
	`
}

// PostgreSQLDialect provides the needed queries for managing schema versioning
// for an PostgreSQL database.
type PostgreSQLDialect struct{}

var _ types.Dialect = PostgreSQLDialect{}

func (PostgreSQLDialect) CreateVersionTableQuery() string {
	return `
		CREATE TABLE
			IF NOT EXISTS schema_version (
				id INTEGER PRIMARY KEY CHECK (id = 0),
				version INTEGER,
				checksum TEXT NOT NULL
			);
	`
}

func (PostgreSQLDialect) CurrentVersionQuery() string {
	return `SELECT id, version, checksum FROM schema_version;`
}

func (PostgreSQLDialect) SaveVersionQuery() string {
	return `
		INSERT INTO schema_version (id, version, checksum)
		VALUES (0, $1, $2)
		ON CONFLICT (id) 
		DO UPDATE SET version = EXCLUDED.version, checksum = EXCLUDED.checksum;
	`
}
