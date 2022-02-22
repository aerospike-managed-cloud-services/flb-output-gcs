// wraps cloud.google.com/go/storage to make it easier to unit test; mostly abstract interfaces

package main //notest

import (
	"context"

	"cloud.google.com/go/storage"
)

type IStorageWriter interface {
	Close() error
	Write(p []byte) (n int, err error)
	SetChunkSize(n int)
}

type storageWriter struct {
	writer *storage.Writer
}

func (stoc *storageWriter) SetChunkSize(n int) {
	stoc.writer.ChunkSize = n
}

func (stoc *storageWriter) Close() error {
	return stoc.writer.Close()
}

func (stoc *storageWriter) Write(p []byte) (n int, err error) {
	return stoc.writer.Write(p)
}

type IStorageClient interface {
	NewWriterFromBucketObjectPath(bucket, path string, ctx context.Context) IStorageWriter
}

type storageClient struct {
	client *storage.Client
}

func (stoc *storageClient) NewWriterFromBucketObjectPath(bucket, path string, ctx context.Context) IStorageWriter {
	writer := stoc.client.Bucket(bucket).Object(path).NewWriter(ctx)
	ret := &storageWriter{writer}
	return ret
}

// StorageAPI abstraction for test
type IStorageAPI interface {
	NewClient(ctx context.Context) (IStorageClient, error)
}

// concrete StorageAPI for production
type storageAPIWrapper struct{}

func (sapi *storageAPIWrapper) NewClient(ctx context.Context) (IStorageClient, error) {
	var cli *storage.Client
	cli, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &storageClient{client: cli}, nil
}
