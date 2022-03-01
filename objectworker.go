package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"text/template"
	"time"

	"github.com/google/uuid"
)

// manages the lifetime of a gcs object
type ObjectWorker struct {
	bucketName         string
	bytesMax           int64
	bufferTimeoutMicro int64
	compression        CompressionType
	timer              *time.Timer
	last               time.Time
	objectPath         string
	tag                string
	objectTemplate     string
	Writer             IStorageWriter
	Written            int64
}

// template input data for constructing the object path
type objectNameData struct {
	InputTag    string
	BeginTime   time.Time
	Dd          string
	IsoDateTime string
	Mm          string
	Timestamp   int64
	Yyyy        string
	Uuid        uuid.UUID
}

// raw struct representation for use in Stringer contexts
func (ond *objectNameData) String() string {
	return fmt.Sprintf("%#v", ond)
}

// constructor
func NewObjectWorker(tag, bucketName, objectTemplate string, sizeKiB int64, timeoutSeconds int, compression CompressionType) *ObjectWorker {
	return &ObjectWorker{
		bucketName:         bucketName,
		bytesMax:           sizeKiB * 1024,
		bufferTimeoutMicro: int64(timeoutSeconds) * 1_000_000,
		compression:        compression,
		tag:                tag,
		objectTemplate:     objectTemplate,
		Written:            0,
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
// we also append ".gz" if the file is gzip-compressed
func (work *ObjectWorker) formatObjectName() string {
	tpl, err := template.New("objectPath").Parse(work.objectTemplate)
	if err != nil { //notest
		logger.Panic().Msgf("Template '%s' could not be parsed", work.objectTemplate)
	}
	buf := new(bytes.Buffer)
	data := objectNameData{
		InputTag:    work.tag,
		IsoDateTime: work.last.UTC().Format("20060102T030405Z"),
		BeginTime:   work.last,
		Timestamp:   work.last.Unix(),
		Yyyy:        fmt.Sprintf("%d", work.last.Year()),
		Mm:          fmt.Sprintf("%02d", work.last.Month()),
		Dd:          fmt.Sprintf("%02d", work.last.Day()),
		Uuid:        uuid.New(),
	}
	if err := tpl.Execute(buf, data); err != nil { //notest
		logger.Panic().Str("template", work.objectTemplate).Stringer("data", &data).Msgf("Template '%s' could not produce a template filename with %#v", work.objectTemplate, data)
	}

	if work.compression == CompressionGzip {
		buf.Write([]byte(".gz"))
	}

	return buf.String()
}

// initialize a writer to write data to a new bucket object
func (work *ObjectWorker) beginStreaming(client IStorageClient) {
	ctx := context.Background()

	work.last = time.Now()
	work.objectPath = work.formatObjectName()

	work.Written = 0

	work.Writer = client.NewWriterFromBucketObjectPath(work.bucketName, work.objectPath, ctx)
	work.Writer.SetChunkSize(256 * 1024) // this is the smallest chunksize you can set and still have buffering

	work.startTimer()
}

// start the idle timer for this worker's write operation
func (work *ObjectWorker) startTimer() {
	expiration := time.Duration(work.bufferTimeoutMicro) * time.Microsecond

	work.timer = time.AfterFunc(expiration, func() {
		dur := expiration.Seconds()
		logger.Debug().Float64("duration", dur).Str("object", work.FormatBucketPath()).Msgf("committing after %.1fs without a commit", dur)
		work.Commit()
	})
}

// write bytes to a worker
func (work *ObjectWorker) Put(client IStorageClient, buf bytes.Buffer) error {
	if work.Writer == nil {
		work.beginStreaming(client)
	}

	// compress the buffer as we go
	var mybuffer bytes.Buffer
	if work.compression == CompressionGzip {
		gzw := gzip.NewWriter(&mybuffer)
		io.Copy(gzw, &buf)
		gzw.Close()
	} else {
		mybuffer = buf
	}

	// copy input buffer to gcs, and account for #bytes written (after compression)
	if written, err := io.Copy(work.Writer, &mybuffer); err != nil {
		return err
	} else {
		work.Written += written
	}

	if work.Written >= work.bytesMax {
		return work.Commit()
	}

	return nil
}

// commit an object being streamed to GCS proper
func (work *ObjectWorker) Commit() error {
	if err := work.Writer.Close(); err != nil {
		return err
	}
	work.timer.Stop()

	logger.Info().Str("object", work.FormatBucketPath()).Float64("kib", float64(work.Written)/1024.0).Msg("committed")

	work.Writer = nil

	return nil
}
