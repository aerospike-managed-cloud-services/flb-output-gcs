package main

import (
	"C"
	"bytes"
	"context"
	"fmt"
	"log"
	"unsafe"

	"cloud.google.com/go/storage"
	"github.com/fluent/fluent-bit-go/output"
)

const (
	FB_OUTPUT_NAME = "gcs"
)

var (
	VERSION    string // to set this, build with --ldflags="-X main.VERSION=vx.y.z"
	the_client *storage.Client
	bucket     *storage.BucketHandle
	workers    map[string](*ObjectWorker)
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
	the_client, err = storage.NewClient(gcsctx)
	if err != nil {
		output.FLBPluginUnregister(plugin)
		log.Fatal(err)
		return output.FLB_ERROR
	}

	// set options from output config with
	ctx := map[string]string{
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
		// inherit security creds from the environment; e.g. be able to use
		//  	application_default_credentials when available creds specified as the
		//  	name of a service account (overrides application_default when specified)
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
	}

	output.FLBPluginSetContext(plugin, ctx)

	// initialize workers
	workers = make(map[string](*ObjectWorker))

	return output.FLB_OK
}

//export FLBPluginFlushCtx
func FLBPluginFlushCtx(ctx, data unsafe.Pointer, length C.int, tag *C.char) int {
	// Gets called with a batch of records to be written to an instance.
	config := output.FLBPluginGetContext(ctx).(map[string]string)

	tag_name := C.GoString(tag)

	work, exists := workers[tag_name]
	if !exists {
		work = NewObjectWorker(tag_name, config["bucket"], config["prefix"])
		workers[tag_name] = work
	}

	dec := output.NewDecoder(data, int(length))
	buf := new(bytes.Buffer)

	for {
		rc, ts, rec := output.GetRecord(dec)
		if rc != 0 {
			break
		}
		timestamp := ts.(output.FLBTime)
		// FIXME display microseconds on timestamp
		buf.WriteString(fmt.Sprintf("[%s] %s: [%d, {", "todo", "todo.0", timestamp.Unix()))
		for key, val := range rec {
			buf.WriteString(fmt.Sprintf("%s: %v, ", key, val))
		}
		buf.WriteString("}]\n")
	}

	log.Printf("[%s] Flush to %s", FB_OUTPUT_NAME, work.FormatBucketPath())

	if err := work.Put(the_client, *buf); err != nil {
		return output.FLB_RETRY
	}

	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	for _, worker := range workers {
		worker.Stop()
	}
	return output.FLB_OK
}

func main() {
}
