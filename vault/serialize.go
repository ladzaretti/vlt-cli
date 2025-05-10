package vault

import (
	"database/sql"
	"fmt"

	"modernc.org/sqlite"
)

// Serialize serializes a binary the SQLite database associated
// with the given *sql.Conn.
//
// The returned buffer contains a complete, self-contained serialization of
// the in-memory database, suitable for use with [Deserialize] or storage.
func Serialize(conn *sql.Conn) (buf []byte, retErr error) {
	err := conn.Raw(func(driverConn any) error {
		c, ok := driverConn.(*sqlite.Conn)
		if !ok {
			return fmt.Errorf("serialize: unexpected driver conn type: %T", driverConn)
		}

		v, err := c.Serialize()
		if err != nil {
			return fmt.Errorf("serialize: %w", err)
		}

		buf = v

		return nil
	})

	return buf, err
}

// Deserialize loads a serialized SQLite database into the given connection.
// The input buffer must be produced by [Serialize].
func Deserialize(conn *sql.Conn, buf []byte) error {
	return conn.Raw(func(driverConn any) error {
		c, ok := driverConn.(*sqlite.Conn)
		if !ok {
			return fmt.Errorf("deserialize: unexpected driverConn type: %T", driverConn)
		}

		err := c.Deserialize(buf)
		if err != nil {
			return fmt.Errorf("deserialize: %w", err)
		}

		return nil
	})
}
