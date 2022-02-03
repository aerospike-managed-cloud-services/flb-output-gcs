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
	bucketName string
	bytesMax   int64
	Closed     bool
	// idleSecondsMax  int
	last       time.Time
	objectPath string
	prefix     string
	tag        string
	writer     *storage.Writer
	Written    int64
}

func NewObjectWorker(tag, bucketName, prefix string, sizeKiB int64) *ObjectWorker {
	last := time.Now()
	objectPath := fmt.Sprintf("%s/%s-%d", prefix, tag, last.Unix())
	return &ObjectWorker{
		bucketName: bucketName,
		bytesMax:   sizeKiB * 1024,
		Closed:     false,
		last:       last,
		objectPath: objectPath,
		prefix:     prefix,
		tag:        tag,
		Written:    0,
	}
}

func (work *ObjectWorker) FormatBucketPath() string {
	return fmt.Sprintf("gs://%s/%s", work.bucketName, work.objectPath)
}

// initialize a writer to write data to the object this worker manages
func (work *ObjectWorker) beginStreaming(client *storage.Client) {
	ctx := context.Background()

	work.last = time.Now()
	work.Written = 0
	work.objectPath = fmt.Sprintf("%s/%s-%d", work.prefix, work.tag, work.last.Unix())
	work.writer = client.Bucket(work.bucketName).Object(work.objectPath).NewWriter(ctx)
	work.writer.ChunkSize = 256 * 1024 // this is the smallest chunksize you can set and still have buffering
}

// write strings to a worker
func (work *ObjectWorker) Put(client *storage.Client, buf bytes.Buffer) error {
	if work.writer == nil {
		work.beginStreaming(client)
	}

	if written, err := io.Copy(work.writer, &buf); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	} else {
		work.Written += written
	}

	work.last = time.Now()

	if work.Written >= work.bytesMax {
		work.roll(client) // FIXME handle error
	}

	return nil
}

// roll over to the next gcs object name
func (work *ObjectWorker) roll(client *storage.Client) error {
	work.writer.Close() // FIXME - this can return error
	prev := work.FormatBucketPath()
	log.Printf("~~ [%s] (%d KiB) rolls over => %s", prev, work.Written/1024, work.FormatBucketPath())

	work.beginStreaming(client)

	return nil
}

func (work *ObjectWorker) Stop() error {
	log.Printf("~~ [%s] Stop()", work.FormatBucketPath())
	err := work.writer.Close()
	work.Closed = true
	return err
}
