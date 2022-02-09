package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"text/template"
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
	tag                  string
	objectTemplate       string
	Writer               *storage.Writer
	Written              int64
}

// template input data for constructing the object path
type objectNameData struct {
	// env string - customerEnvironment
	InputTag string
	// instanceName string - hostname of the log source
	// instanceId string - cloud provider internal id of the host that produced the log
	BeginTime   time.Time
	Dd          string
	IsoDateTime string
	Mm          string
	Timestamp   int64
	Yyyy        string
}

func NewObjectWorker(tag, bucketName, objectTemplate string, sizeKiB int64, timeoutSeconds int) *ObjectWorker {
	return &ObjectWorker{
		bucketName:           bucketName,
		bytesMax:             sizeKiB * 1024,
		bufferTimeoutSeconds: timeoutSeconds,
		tag:                  tag,
		objectTemplate:       objectTemplate,
		Written:              0,
	}
}

// produce a gs:// url for the object being written.
// if no object is currently being written, substitute "[closed]" in place of
// object name.
func (work *ObjectWorker) FormatBucketPath() string {
	if work.Writer != nil {
		return fmt.Sprintf("gs://%s/%s", work.bucketName, work.objectPath)

	}
	return "[closed]"
}

// set the Worker objectPath by applying the template to the current time and input tag
func (work *ObjectWorker) formatObjectName() string {
	tpl, err := template.New("objectPath").Parse(work.objectTemplate)
	if err != nil {
		log.Panicf("Template '%s' could not be parsed", work.objectTemplate)
	}
	buf := new(bytes.Buffer)
	data := objectNameData{
		InputTag: work.tag,
		// instanceName
		// instanceId
		IsoDateTime: work.last.UTC().Format("20060102T030405Z"),
		BeginTime:   work.last,
		Timestamp:   work.last.Unix(),
		Yyyy:        fmt.Sprintf("%d", work.last.Year()),
		Mm:          fmt.Sprintf("%02d", work.last.Month()),
		Dd:          fmt.Sprintf("%02d", work.last.Day()),
	}
	if err := tpl.Execute(buf, data); err != nil {
		log.Panicf("Template '%s' could not produce a template filename with %#v", work.objectTemplate, data)
	}
	return buf.String()
}

// initialize a writer to write data to a new bucket object
func (work *ObjectWorker) beginStreaming(client *storage.Client) {
	ctx := context.Background()

	work.last = time.Now()
	work.objectPath = work.formatObjectName()

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
