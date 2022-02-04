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
	bucketName           string
	bytesMax             int64
	bufferTimeoutSeconds int
	timer                *time.Timer
	last                 time.Time
	objectPath           string
	prefix               string
	tag                  string
	Writer               *storage.Writer
	Written              int64
}

func NewObjectWorker(tag, bucketName, prefix string, sizeKiB int64, timeoutSeconds int) *ObjectWorker {
	return &ObjectWorker{
		bucketName:           bucketName,
		bytesMax:             sizeKiB * 1024,
		bufferTimeoutSeconds: timeoutSeconds,
		prefix:               prefix,
		tag:                  tag,
		Written:              0,
	}
}

func (work *ObjectWorker) FormatBucketPath() string {
	if work.Writer != nil {
		return fmt.Sprintf("gs://%s/%s", work.bucketName, work.objectPath)

	}
	return fmt.Sprintf("gs://%s/%s/[closed]", work.bucketName, work.prefix)
}

// initialize a writer to write data to the object this worker manages
func (work *ObjectWorker) beginStreaming(client *storage.Client) {
	ctx := context.Background()

	work.last = time.Now()
	work.objectPath = fmt.Sprintf("%s/%s-%d", work.prefix, work.tag, work.last.Unix())
	work.Written = 0

	work.Writer = client.Bucket(work.bucketName).Object(work.objectPath).NewWriter(ctx)
	work.Writer.ChunkSize = 256 * 1024 // this is the smallest chunksize you can set and still have buffering

	work.startTimer(client)
}

// start the idle timer for this worker's write operation
func (work *ObjectWorker) startTimer(client *storage.Client) {
	expiration := time.Duration(work.bufferTimeoutSeconds) * time.Second

	work.timer = time.AfterFunc(expiration, func() {
		log.Printf("[%s] %.1fs without a commit, going to commit", work.FormatBucketPath(), expiration.Seconds())
		work.Commit(client)
	})
}

// write strings to a worker
func (work *ObjectWorker) Put(client *storage.Client, buf bytes.Buffer) error {
	if work.Writer == nil {
		work.beginStreaming(client)
	}

	if written, err := io.Copy(work.Writer, &buf); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	} else {
		work.Written += written
	}

	if work.Written >= work.bytesMax {
		work.Commit(client) // FIXME handle error
	}

	return nil
}

// commit an object being streamed to GCS proper
func (work *ObjectWorker) Commit(client *storage.Client) error {
	work.Writer.Close() // FIXME - this can return error
	work.timer.Stop()

	log.Printf("~~ [%s] (%.1f KiB) committed", work.FormatBucketPath(), float64(work.Written)/1024.0)

	work.Writer = nil

	return nil
}
