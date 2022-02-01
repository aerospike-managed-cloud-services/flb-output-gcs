package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"cloud.google.com/go/storage"
)

// manages the lifetime of a gcs object
type ObjectWorker struct {
	tag         string
	bucket_name string
	object_path string
	last        time.Time
	written     int64
	builder     strings.Builder
	writer      *storage.Writer
}

func NewObjectWorker(tag string, bucket_name, prefix string) *ObjectWorker {
	last := time.Now()
	object_path := fmt.Sprintf("%s/%s-%d", prefix, tag, last.Unix())
	return &ObjectWorker{
		tag:         tag,
		bucket_name: bucket_name,
		object_path: object_path,
		last:        last,
		written:     0,
		builder:     strings.Builder{},
	}
}

func (work *ObjectWorker) FormatBucketPath() string {
	return fmt.Sprintf("gs://%s/%s", work.bucket_name, work.object_path)
}

// FIXME - we'll remove this when streaming works
// // upload log text to an object, clobbering whatever was there before
// func (work *ObjectWorker) Clobber(client *storage.Client, buf bytes.Buffer) (int, error) {
// 	ctx := context.Background()
// 	wc := client.Bucket(work.bucket_name).Object(work.object_path).NewWriter(ctx)
// 	n, err := wc.Write(buf.Bytes())
// 	wc.Close()
// 	return n, err
// }

// initialize a writer to write data to the object this worker manages
func (work *ObjectWorker) beginStreaming(client *storage.Client) {
	ctx := context.Background()

	work.writer = client.Bucket(work.bucket_name).Object(work.object_path).NewWriter(ctx)
	work.writer.ChunkSize = 256 * 1024 // this is the smallest chunksize you can set and still have buffering
}

// write strings to a worker
func (work *ObjectWorker) Put(client *storage.Client, buf bytes.Buffer) error {
	if work.writer == nil {
		work.beginStreaming(client)
	}

	if _, err := io.Copy(work.writer, &buf); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}

	work.last = time.Now()

	return nil
}

// roll over to the next gcs object name
func (work *ObjectWorker) roll() error {
	return nil
}

func (work *ObjectWorker) Stop() error {
	return work.writer.Close()
}
