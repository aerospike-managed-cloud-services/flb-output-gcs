package main

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

func (sto *storageWriter) SetChunkSize(n int) {
	sto.writer.ChunkSize = n
}

func (sto *storageWriter) Close() error {
	return sto.writer.Close()
}

func (sto *storageWriter) Write(p []byte) (n int, err error) {
	return sto.writer.Write(p)
}

type IStorageClient interface {
	NewWriterFromBucketObjectPath(bucket, path string, ctx context.Context) IStorageWriter
}

type storageClient struct {
	client *storage.Client
}

func (sto *storageClient) NewWriterFromBucketObjectPath(bucket, path string, ctx context.Context) IStorageWriter {
	writer := sto.client.Bucket(bucket).Object(path).NewWriter(ctx)
	ret := &storageWriter{writer}
	return ret
}

func NewStorageClient(ctx context.Context) (*storageClient, error) {
	var cli *storage.Client
	cli, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &storageClient{client: cli}, nil
}
