package compression

import (
	"bytes"
	"compress/gzip"
	"io"
)

type GzipCompressor struct{}

func (g GzipCompressor) Compress(data []byte) ([]byte, error) {
	var b bytes.Buffer
	writer := gzip.NewWriter(&b)
	_, err := writer.Write(data)
	if err != nil {
		writer.Close()
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (g GzipCompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
