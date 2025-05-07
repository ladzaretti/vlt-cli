package vlt

import (
	"database/sql"
	"testing"
)

func TestSerialize(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create a table and insert data
	//
	_, err = db.Exec(`CREATE TABLE foo (msg TEXT NOT NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO foo VALUES ('bar');`)
	if err != nil {
		t.Fatal(err)
	}

	// Serialize the data
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	data, err := Serialize(conn)
	if err != nil {
		t.Fatal("Serialize:", err)
	}
	conn.Close()

	// Open a new in-memory database
	db2, err := sql.Open("sqlite", ":memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	// Get second connection and try to query before deserialization
	conn2, err := db2.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()

	var msg string
	err = conn2.QueryRowContext(t.Context(), `SELECT msg FROM foo`).Scan(&msg)
	if err == nil {
		t.Fatal("Query:", err)
	}

	// Deserialize the data into the second connection
	err = Deserialize(conn2, data)
	if err != nil {
		t.Fatal("Deserialize:", err)
	}

	// Check that the data is correctly deserialized
	err = conn2.QueryRowContext(t.Context(), `SELECT msg FROM foo`).Scan(&msg)
	if err != nil {
		t.Fatal("Query:", err)
	}
	if msg != "bar" {
		t.Fatalf("unexpected msg: %q", msg)
	}
}
