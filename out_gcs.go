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
	VERSION   string                    // to set this, build with --ldflags="-X main.VERSION=vx.y.z"
	instances map[string](*outputState) = make(map[string](*outputState))
)

type outputState struct {
	// name of the bucket
	// required, no default
	bucket string

	// maximum size (in KiB) held in the request Writer buffer before committing an object to the bucket
	// default 5000
	bufferSizeKiB int64

	// maximum time (in s) between writes before the request Writer must commit to the bucket
	// (even if bufferSizeKiB has not been reached)
	// default 300
	bufferTimeoutSeconds int

	// compression type, allowed values: none; gzip
	// default "none"
	compression CompressionType

	// internal-use; connectable google storage api client
	gcsClient *storage.Client

	// string to uniquely identify this output plugin instance
	outputID string

	// a template for the object filename that gets created in the bucket. this uses golang text/template syntax.
	// The following placeholders are recognized:
	// {{ .InputTag }} the tag of the associated fluent "input" being flushed, e.g. "cpu"
	// {{ .Timestamp }} timestamp using unix seconds since 1970-01-01
	// {{ .IsoDateTime }} 14-digit YYYYmmddTHHMMSSZ datetime format, UTC
	// {{ .Yyyy }} {{ .Mm }} {{ .Dd }} year, month, day
	// {{ .BeginTime.Format "2006...." }} .beginTime is a time.Time() object and you can use any method on it;
	// 								      for example, you can call .Format() as shown and get any format you want
	// The object created will be in gs://BUCKET/
	// default "{{ .InputTag }}-{{ .Timestamp }}
	objectNameTemplate string

	// internal-use; map of inputTag to a gcs api client worker
	workers map[string](*ObjectWorker)
}

type flbOutputAPIWrapper struct {
}

func (*flbOutputAPIWrapper) FLBPluginConfigKey(plugin unsafe.Pointer, skey string) string {
	return output.FLBPluginConfigKey(plugin, skey)
}

func (*flbOutputAPIWrapper) FLBPluginRegister(plugin unsafe.Pointer, name string, desc string) int {
	return output.FLBPluginRegister(plugin, name, desc)
}

func (*flbOutputAPIWrapper) FLBPluginUnregister(plugin unsafe.Pointer) {
	output.FLBPluginUnregister(plugin)
}

func (*flbOutputAPIWrapper) FLBPluginSetContext(plugin unsafe.Pointer, ctx interface{}) {
	output.FLBPluginSetContext(plugin, ctx)
}

func (*flbOutputAPIWrapper) FLBPluginGetContext(proxyContext unsafe.Pointer) interface{} {
	return output.FLBPluginGetContext(proxyContext)
}

func (*flbOutputAPIWrapper) NewDecoder(data unsafe.Pointer, length int) *output.FLBDecoder {
	return output.NewDecoder(data, length)
}

func (*flbOutputAPIWrapper) GetRecord(dec *output.FLBDecoder) (ret int, ts interface{}, rec map[interface{}]interface{}) {
	return output.GetRecord(dec)
}

func NewFLBOutputAPI() *flbOutputAPIWrapper {
	return &flbOutputAPIWrapper{}
}

var flbAPI IFLBOutputAPI = NewFLBOutputAPI()

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	description := fmt.Sprintf("GCS bucket output %s", VERSION)
	return flbAPI.FLBPluginRegister(def, FB_OUTPUT_NAME, description)
}

// convert a plugin config string to int or return (, false) to accept the default
func pluginConfigValueToInt(plugin unsafe.Pointer, skey string) (int64, bool) {
	sval := flbAPI.FLBPluginConfigKey(plugin, skey)

	// empty -> use the default
	if sval == "" {
		return 0, false
	}

	if v, err := strconv.ParseInt(sval, 10, 64); err != nil {
		log.Printf("** Warning: '%s %s' was not an integer, using default", skey, sval)
		// can't parse; warn, and use the default
		return 0, false
	} else {
		return v, true
	}
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	// [OUTPUT] sections for the gcs plugin must have an id field
	outputID := flbAPI.FLBPluginConfigKey(plugin, "OutputID")
	if outputID == "" {
		flbAPI.FLBPluginUnregister(plugin)
		log.Fatal("[gcs] 'OutputID' is a required field and is missing from 1 or more [output] blocks. Check your .conf and add this field.")
		return output.FLB_ERROR
	}

	// create a GCS API client for this output instance, or die
	gcsctx := context.Background()
	client, err := storage.NewClient(gcsctx)
	if err != nil {
		flbAPI.FLBPluginUnregister(plugin)
		log.Fatal(err)
		return output.FLB_ERROR
	}

	// parse configuration for this output instance
	ost := outputState{
		bucket:               flbAPI.FLBPluginConfigKey(plugin, "Bucket"),
		bufferSizeKiB:        5000,
		bufferTimeoutSeconds: 300,
		compression:          CompressionNone,
		gcsClient:            client,
		outputID:             outputID,
		objectNameTemplate:   flbAPI.FLBPluginConfigKey(plugin, "ObjectNameTemplate"),

		// initialize workers; this instance will eventually add 1 worker per input to this map
		workers: map[string]*ObjectWorker{},
	}

	if ost.objectNameTemplate == "" {
		ost.objectNameTemplate = "{{ .InputTag }}-{{ .Timestamp }}"
	}

	if bskb, ok := pluginConfigValueToInt(plugin, "BufferSizeKiB"); ok {
		ost.bufferSizeKiB = bskb
	}

	if bts, ok := pluginConfigValueToInt(plugin, "BufferTimeoutSeconds"); ok {
		ost.bufferTimeoutSeconds = int(bts)
	}

	if cmpr := flbAPI.FLBPluginConfigKey(plugin, "Compression"); cmpr != "" {
		switch CompressionType(cmpr) {
		case CompressionNone:
			ost.compression = CompressionNone
		case CompressionGzip:
			ost.compression = CompressionGzip
		default:
			log.Printf("** Warning: 'Compression %s' should be 'gzip' or 'none'; using default", cmpr)
		}
	}

	instances[ost.outputID] = &ost

	flbAPI.FLBPluginSetContext(plugin, ost)

	return output.FLB_OK
}

//export FLBPluginFlushCtx
func FLBPluginFlushCtx(ctx, data unsafe.Pointer, length C.int, tag *C.char) int {
	state := flbAPI.FLBPluginGetContext(ctx).(outputState)

	tag_name := C.GoString(tag)

	work, exists := state.workers[tag_name]
	if !exists {
		work = NewObjectWorker(
			tag_name,
			state.bucket,
			state.objectNameTemplate,
			state.bufferSizeKiB,
			state.bufferTimeoutSeconds,
			state.compression,
		)
		state.workers[tag_name] = work
	}

	dec := flbAPI.NewDecoder(data, int(length))
	buf := new(bytes.Buffer)

	// Gets called with a batch of records to be written to an instance.
	// Decode each rec
	for {
		// FIXME - when we call this through the interface, this works
		// but the test fails (even if we don't call GetRecord)
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

	if err := work.Put(state.gcsClient, *buf); err != nil {
		return output.FLB_RETRY
	}

	log.Printf("[%s] Flushed %s (%db)", FB_OUTPUT_NAME, work.FormatBucketPath(), work.Written)

	return output.FLB_OK
}

// DO NOT USE.
//
// FLBPluginExitCtx is called once per output instance but is ONLY passed the context
// for the first instance (potentially multiple times, same argument).
//
// This appears to be a bug in FLBPluginExitCtx
// https://github.com/fluent/fluent-bit-go/issues/49
//
// func FLBPluginExitCtx(ctx unsafe.Pointer) int {
// 	return output.FLB_OK
// }

//export FLBPluginExit
func FLBPluginExit() int {
	for _, inst := range instances {
		for _, worker := range inst.workers {
			// due to the FLBPluginExitCtx bug (see comment above), we just have
			// to check and see whether each one is closed here.
			if worker.Writer != nil {
				worker.Commit(inst.gcsClient)
			}
		}
	}
	return output.FLB_OK
}

func main() {
}
