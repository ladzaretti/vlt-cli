package main_test

import "testing"

func TestMain(t *testing.T) {
	b := true

	if !b {
		t.Error("Dummy test")
	}
}
