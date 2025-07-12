package genericclioptions

import (
	"bytes"
	"os"
	"time"
)

type TestFdReader struct {
	*bytes.Buffer

	fd uintptr

	fi os.FileInfo
}

func NewTestFdReader(b *bytes.Buffer, fd uintptr, fi os.FileInfo) *TestFdReader {
	return &TestFdReader{
		Buffer: b,
		fd:     fd,
		fi:     fi,
	}
}

var _ FdReader = &TestFdReader{}

func (r *TestFdReader) Fd() uintptr {
	return r.fd
}

func (r *TestFdReader) Stat() (os.FileInfo, error) {
	return r.fi, nil
}

type testFileInfo struct {
	name  string
	size  int64
	mode  os.FileMode
	t     time.Time
	isDir bool
}

func NewMockFileInfo(name string, size int64, mode os.FileMode, isDir bool, t time.Time) os.FileInfo {
	return &testFileInfo{
		name:  name,
		size:  size,
		mode:  mode,
		isDir: isDir,
		t:     t,
	}
}

func (fi *testFileInfo) Name() string       { return fi.name }
func (fi *testFileInfo) Size() int64        { return fi.size }
func (fi *testFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *testFileInfo) ModTime() time.Time { return fi.t }
func (fi *testFileInfo) IsDir() bool        { return fi.isDir }
func (*testFileInfo) Sys() any              { return nil }
