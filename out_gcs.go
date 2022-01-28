package main

import (
	"C"
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"github.com/fluent/fluent-bit-go/output"
	"log"
	"strings"
	"unsafe"
)

const (
	FB_OUTPUT_NAME = "gcs"
)

var (
	VERSION string // to set this, build with --ldflags="-X main.VERSION=vx.y.z"
	client  *storage.Client
)

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	description := fmt.Sprintf("GCS bucket output %s", VERSION)
	return output.FLBPluginRegister(def, FB_OUTPUT_NAME, description)
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	gcsctx := context.Background()
	var err error
	client, err = storage.NewClient(gcsctx)
	if err != nil {
		output.FLBPluginUnregister(plugin)
		log.Fatal(err)
		return output.FLB_ERROR
	}

	// set options from output config with
	output.FLBPluginSetContext(plugin, map[string]string{
		// name of the bucket
		// no default
		"bucket": output.FLBPluginConfigKey(plugin, "Bucket"),

		// bucket prefix, i.e. path
		// no default
		"prefix": output.FLBPluginConfigKey(plugin, "Prefix"),

		// GCP project that owns the bucket
		// no default (if unset, use inherited project from the environment)
		"project": output.FLBPluginConfigKey(plugin, "Project"),

		// service account in the specified project that I will use to access the bucket
		// no default (if unset, use inherited credentials from the environment)
		"serviceAccount": output.FLBPluginConfigKey(plugin, "ServiceAccount"),

		// maximum size (in KiB) held in memory before a chunk is written to the bucket object
		// default 5000
		// "bufferSizeKiB": output.FLBPluginConfigKey(plugin, "BufferSizeKiB")

		// maximum time (in s) between writes before a write to the bucket object must occur
		// (even if bufferSizeKiB has not been reached)
		// default 300
		// "bufferTimeoutSeconds": output.FLBPluginConfigKey(plugin, "BufferTimeoutSeconds")

		// maximum size (in KiB) a bucket object can reach before rolling over to a new object
		// default 100000
		// "objectRolloverKiB": output.FLBPluginConfigKey(plugin, "ObjectRolloverKiB")
	})

	// inherit security creds from the environment; e.g. be able to use
	// 	application_default_credentials when available creds specified as the
	// 	name of a service account (overrides application_default when specified)
	// when 5 minutes have expired with no new log buffered

	return output.FLB_OK
}

//export FLBPluginFlushCtx
func FLBPluginFlushCtx(ctx, data unsafe.Pointer, length C.int, tag *C.char) int {
	// Gets called with a batch of records to be written to an instance.

	config := output.FLBPluginGetContext(ctx).(map[string]string)
	dec := output.NewDecoder(data, int(length))
	for {
		rc, ts, rec := output.GetRecord(dec)
		if rc != 0 {
			break
		}
		timestamp := ts.(output.FLBTime)
		var printable strings.Builder
		printable.WriteString(fmt.Sprintf("[%s, {", timestamp))
		for key, val := range rec {
			printable.WriteString(fmt.Sprintf("%s: %v, ", key, val))
		}
		printable.WriteString("}\n")

		log.Print(printable.String())
		obj_filename := "TODO_FILENAME"
		log.Printf("[%s] Flush to gs://%s/%s/%s", FB_OUTPUT_NAME, config["bucket"], config["prefix"], obj_filename)

	}

	// FLB_ERROR does not retry these bytes
	// FLB_RETRY does retry these bytes
	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

// // streamFileUpload uploads an object via a stream.
// func streamFileUpload(w io.Writer, bucket, object string) error {
// 	// bucket := "bucket-name"
// 	// object := "object-name"
// 	ctx := context.Background()
// 	client, err := storage.NewClient(ctx)
// 	if err != nil {
// 		return fmt.Errorf("storage.NewClient: %v", err)
// 	}
// 	defer client.Close()
//
// 	b := []byte("Hello world.")
// 	buf := bytes.NewBuffer(b)
//
// 	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
// 	defer cancel()
//
// 	// Upload an object with storage.Writer.
// 	wc := client.Bucket(bucket).Object(object).NewWriter(ctx)
// 	wc.ChunkSize = 0 // note retries are not supported for chunk size 0.
//
// 	if _, err = io.Copy(wc, buf); err != nil {
// 		return fmt.Errorf("io.Copy: %v", err)
// 	}
// 	// Data can continue to be added to the file until the writer is closed.
// 	if err := wc.Close(); err != nil {
// 		return fmt.Errorf("Writer.Close: %v", err)
// 	}
//
// 	return nil
// }

func main() {
}
