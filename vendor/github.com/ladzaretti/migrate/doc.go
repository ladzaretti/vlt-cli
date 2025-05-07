// Package migrate provides a generic and database-agnostic schema migration tool.
//
// It works with any SQL (or SQL-like) database that has a [database/sql] driver.
// See https://go.dev/wiki/SQLDrivers for a list of supported drivers.
//
// Migrations are versioned, transactional (when supported), and verified using checksums
// to detect changes in already applied scripts. PostgreSQL and SQLite are supported
// out of the box, with the ability to extend support for additional dialects.
package migrate
