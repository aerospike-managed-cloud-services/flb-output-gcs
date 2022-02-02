package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"cloud.google.com/go/storage"
)

// manages the lifetime of a gcs object
type ObjectWorker struct {
	bucket_name     string
	last            time.Time
	object_path     string
	prefix          string
	tag             string
	writer          *storage.Writer
	written         int64
	PUT_COUNT_FIXME int
}

func NewObjectWorker(tag string, bucket_name, prefix string) *ObjectWorker {
	last := time.Now()
	object_path := fmt.Sprintf("%s/%s-%d", prefix, tag, last.Unix())
	return &ObjectWorker{
		bucket_name:     bucket_name,
		last:            last,
		object_path:     object_path,
		prefix:          prefix,
		tag:             tag,
		written:         0,
		PUT_COUNT_FIXME: 0,
	}
}

func (work *ObjectWorker) FormatBucketPath() string {
	return fmt.Sprintf("gs://%s/%s", work.bucket_name, work.object_path)
}

// initialize a writer to write data to the object this worker manages
func (work *ObjectWorker) beginStreaming(client *storage.Client) error {
	ctx := context.Background()

	work.writer = client.Bucket(work.bucket_name).Object(work.object_path).NewWriter(ctx) // TODO errors

	work.writer.ChunkSize = 256 * 1024 // this is the smallest chunksize you can set and still have buffering

	return nil
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

	work.PUT_COUNT_FIXME += 1
	log.Printf("~~ [%s].PUT_COUNT_FIXME = %d", work.FormatBucketPath(), work.PUT_COUNT_FIXME)

	if work.PUT_COUNT_FIXME >= 8 {
		work.roll(client)
		work.PUT_COUNT_FIXME = 0
	}

	return nil
}

// roll over to the next gcs object name
func (work *ObjectWorker) roll(client *storage.Client) error {
	prev := work.FormatBucketPath()
	work.writer.Close()
	work.last = time.Now()
	work.object_path = fmt.Sprintf("%s/%s-%d", work.prefix, work.tag, work.last.Unix())
	log.Printf("~~ [%s] rolls over => %s", prev, work.FormatBucketPath())

	return work.beginStreaming(client)
}

func (work *ObjectWorker) Stop() error {
	log.Printf("~~ [%s] Stop()", work.FormatBucketPath())
	return work.writer.Close()
}
