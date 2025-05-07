package migrate

import (
	"embed"
	"path/filepath"
)

// Lister is an interface that defines a method for listing
// the contents of the underlying data source.
type Lister interface {
	List() ([]string, error)
}

// StringMigrations is a slice of plain string migration script queries to be applied.
type StringMigrations []string

func (s StringMigrations) List() ([]string, error) {
	return s, nil
}

// EmbeddedMigrations wraps the [embed.FS] and the path to the migration scripts directory.
type EmbeddedMigrations struct {
	FS   embed.FS
	Path string
}

// List returns a list of migration script queries from the embedded file system.
//
// It reads migration scripts from the directory specified
// in the [EmbeddedMigrations.Path] field within the embedded file system [EmbeddedMigrations.FS]
// and returns them as a slice of strings.
//
// This function does not recursively read subdirectories.
//
// Queries are ordered lexicographically rather than naturally.
// For example, the files "1.sql", "2.sql", and "03.sql"
// will be read in the order: "03.sql", "1.sql", "2.sql".
//
// To ensure correct ordering, use zero-padding for numbers, e.g.,
// "001.sql", "002.sql", "003.sql".
func (e EmbeddedMigrations) List() ([]string, error) {
	files, err := e.FS.ReadDir(e.Path)
	if err != nil {
		return nil, errf("reading embedded migration directory: %v", err)
	}

	ss := make([]string, 0, len(files))

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		p := filepath.Join(e.Path, f.Name())

		s, err := e.FS.ReadFile(p)
		if err != nil {
			return nil, errf("reading embedded migration file: %v", err)
		}

		ss = append(ss, string(s))
	}

	return ss, nil
}
