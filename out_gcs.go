package main

import (
	"C"
	"bytes"
	"context"
	"fmt"
	"log"
	"strconv"
	"unsafe"

	"cloud.google.com/go/storage"
	"github.com/fluent/fluent-bit-go/output"
)

const (
	FB_OUTPUT_NAME = "gcs"
)

type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionGzip CompressionType = "gzip"
)

var (
	VERSION    string // to set this, build with --ldflags="-X main.VERSION=vx.y.z"
	the_client *storage.Client
	bucket     *storage.BucketHandle
	workers    map[string](*ObjectWorker)
)

type config struct {
	bucket               string
	prefix               string
	project              string
	serviceAccount       string
	bufferSizeKiB        int
	bufferTimeoutSeconds int
	compression          CompressionType
	// objectNameTemplate   string
}

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	description := fmt.Sprintf("GCS bucket output %s", VERSION)
	return output.FLBPluginRegister(def, FB_OUTPUT_NAME, description)
}

// convert a plugin config string to int or return (, false) to accept the default
func pluginConfigValueToInt(plugin unsafe.Pointer, skey string) (int, bool) {
	sval := output.FLBPluginConfigKey(plugin, skey)

	// empty -> use the default
	if sval == "" {
		return 0, false
	}

	if v, err := strconv.Atoi(sval); err != nil {
		log.Printf("** Warning: '%s %s' was not an integer, using default", skey, sval)
		// can't parse; warn, and use the default
		return 0, false
	} else {
		return v, true
	}
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

	ctx := config{
		// name of the bucket
		// required, no default
		bucket: output.FLBPluginConfigKey(plugin, "Bucket"),

		// bucket prefix, i.e. path
		// default ""
		prefix: output.FLBPluginConfigKey(plugin, "Prefix"),

		// GCP project that owns the bucket
		// not required, no default (if unset, use inherited project from the environment)
		project: output.FLBPluginConfigKey(plugin, "Project"),

		// service account in the specified project that I will use to access the bucket
		// no default (if unset, use inherited credentials from the environment)
		// inherit security creds from the environment; e.g. be able to use
		//  	application_default_credentials when available creds specified as the
		//  	name of a service account (overrides application_default when specified)
		serviceAccount: output.FLBPluginConfigKey(plugin, "ServiceAccount"),

		// maximum size (in KiB) held in memory before an object is written to a bucket
		// default 5000
		bufferSizeKiB: 5000,

		// maximum time (in s) between writes before a write to the bucket object must occur
		// (even if bufferSizeKiB has not been reached)
		// default 300
		bufferTimeoutSeconds: 300,

		// compression type, allowed values: none; gzip
		// default "none"
		compression: CompressionNone,

		// // a template for the object filename that gets created in the bucket. following placeholders are recognized:
		// // ${inputTag}, ${unixTimeStamp}, ${isoDateTime}, ...
		// // The object created will be in gs://BUCKET/PREFIX/
		// // default "${inputTag}-${unixTimeStamp}"
		// objectNameTemplate: "${inputTag}-${unixTimeStamp}",
	}

	if bskb, ok := pluginConfigValueToInt(plugin, "BufferSizeKiB"); ok {
		ctx.bufferSizeKiB = bskb
	}

	if bts, ok := pluginConfigValueToInt(plugin, "BufferTimeoutSeconds"); ok {
		ctx.bufferTimeoutSeconds = bts
	}

	if cmpr := output.FLBPluginConfigKey(plugin, "Compression"); cmpr != "" {
		switch CompressionType(cmpr) {
		case CompressionNone:
			ctx.compression = CompressionNone
		case CompressionGzip:
			ctx.compression = CompressionGzip
		default:
			log.Printf("** Warning: 'Compression %s' should be 'gzip' or 'none'; using default", cmpr)
		}
	}

	output.FLBPluginSetContext(plugin, ctx)

	// initialize workers
	workers = make(map[string](*ObjectWorker))

	return output.FLB_OK
}

//export FLBPluginFlushCtx
func FLBPluginFlushCtx(ctx, data unsafe.Pointer, length C.int, tag *C.char) int {
	// Gets called with a batch of records to be written to an instance.
	cfg := output.FLBPluginGetContext(ctx).(config)

	tag_name := C.GoString(tag)

	work, exists := workers[tag_name]
	if !exists {
		work = NewObjectWorker(tag_name, cfg.bucket, cfg.prefix, cfg.bufferSizeKiB)
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
