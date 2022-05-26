package utils

import (
	"bytes"
	"compress/gzip"
)

func Gzip(byteArr []byte) []byte {
	b := &bytes.Buffer{}
	w := gzip.NewWriter(b)
	w.Write(byteArr)
	w.Close()
	return b.Bytes()
}
