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
	bucketName      string
	last            time.Time
	object_path     string
	prefix          string
	tag             string
	writer          *storage.Writer
	written         int64
	PUT_COUNT_FIXME int
	buffer          *bytes.Buffer
}

func NewObjectWorker(tag, bucketName, prefix string, size int) *ObjectWorker {
	last := time.Now()
	object_path := fmt.Sprintf("%s/%s-%d", prefix, tag, last.Unix())
	return &ObjectWorker{
		bucketName: bucketName,
		// pad the bytes buffer by 5k to help reduce allocations near the boundary of a rollover
		buffer:          bytes.NewBuffer(make([]byte, (size+5)*1024)),
		last:            last,
		object_path:     object_path,
		prefix:          prefix,
		PUT_COUNT_FIXME: 0,
		tag:             tag,
		written:         0,
	}
}

func (work *ObjectWorker) FormatBucketPath() string {
	return fmt.Sprintf("gs://%s/%s", work.bucketName, work.object_path)
}

// initialize a writer to write data to the object this worker manages
func (work *ObjectWorker) beginStreaming(client *storage.Client) {
	ctx := context.Background()

	work.writer = client.Bucket(work.bucketName).Object(work.object_path).NewWriter(ctx) // TODO errors

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
	work.writer.Close() // FIXME - this can return error
	work.last = time.Now()
	work.object_path = fmt.Sprintf("%s/%s-%d", work.prefix, work.tag, work.last.Unix())
	log.Printf("~~ [%s] rolls over => %s", prev, work.FormatBucketPath())

	work.beginStreaming(client)

	return nil
}

func (work *ObjectWorker) Stop() error {
	log.Printf("~~ [%s] Stop()", work.FormatBucketPath())
	return work.writer.Close()
}
