package save

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"testing"
)

// plain is path's JSON, gunzipped.
func plain(t *testing.T, path string) []byte {
	t.Helper()
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	zr, err := gzip.NewReader(fh)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// writePlain gzips b to path, as Write does.
func writePlain(t *testing.T, path string, b []byte) {
	t.Helper()
	var z bytes.Buffer
	zw := gzip.NewWriter(&z)
	zw.Write(b)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, z.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
