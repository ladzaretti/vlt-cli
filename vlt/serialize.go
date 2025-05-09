package vlt

import (
	"database/sql"
	"fmt"

	"modernc.org/sqlite"
)

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

// Deserialize restores the contents of the database connected via this
// Conn from the given serialized buffer. The buffer should be produced by a
// call to [Conn.Serialize].
func Deserialize(conn *sql.Conn, buf []byte) error {
	return conn.Raw(func(driverConn any) error {
		c, ok := driverConn.(*sqlite.Conn)
		if !ok {
			return fmt.Errorf("deserialize: unexpected driverConn type: %T", driverConn)
		}

		err := c.DeserializeWithFlags(buf, 0)
		if err != nil {
			return fmt.Errorf("deserialize: %w", err)
		}

		return nil
	})
}
