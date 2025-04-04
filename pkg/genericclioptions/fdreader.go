package genericclioptions

import (
	"io"
	"os"
)

// FdReader defines the interface for file-like objects that can be read from,
// provide a file descriptor, and return file information.
type FdReader interface {
	Fd() uintptr
	Stat() (os.FileInfo, error)

	io.Reader
}
