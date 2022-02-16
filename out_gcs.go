package main

import (
	"C"
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"unsafe"

	"cloud.google.com/go/storage"
	"github.com/fluent/fluent-bit-go/output"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	FB_OUTPUT_NAME = "gcs"
)

// gzip or none
type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionGzip CompressionType = "gzip"
)

var (
	VERSION   string                    // to set this, build with --ldflags="-X main.VERSION=vx.y.z"
	instances map[string](*outputState) = make(map[string](*outputState))
)

// Settings for this output plugin instance.
//
// The workers map creates one worker per input being handled by this instance.
// The organization of fluent-bit permits multiple output plugins routing
// different data to different places, but within each stream of events
// you can have multiple inputs, each of which gets its own worker here.
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

// global access to the fluent-bit API through this object
var flbAPI IFLBOutputAPI = NewFLBOutputAPI()

// structured data logger; this constructor makes it easier to replace in a test
var logger zerolog.Logger = log.Logger.With().Str("output", FB_OUTPUT_NAME).Logger()

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
		logger.Warn().Str(skey, sval).Msg("option value should be an int, using default")
		// can't parse; warn, and use the default
		return 0, false
	} else {
		return v, true
	}
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	// change the logging style for dev testing
	if dev := os.Getenv("OUT_GCS_DEV_LOGGING"); dev != "" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// [OUTPUT] sections for the gcs plugin must have an id field
	outputID := flbAPI.FLBPluginConfigKey(plugin, "OutputID")
	if outputID == "" {
		flbAPI.FLBPluginUnregister(plugin)
		logger.Fatal().Str("field", "OutputID").Msg("a required field is missing from 1 or more [output] blocks. Check your .conf and add this field.")
		return output.FLB_ERROR
	}

	// create a GCS API client for this output instance, or die
	gcsctx := context.Background()
	client, err := storage.NewClient(gcsctx)
	if err != nil {
		flbAPI.FLBPluginUnregister(plugin)
		logger.Fatal().Msg(err.Error())
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
			logger.Warn().Msgf("'Compression %s' should be 'gzip' or 'none'; using default", cmpr)
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

	logger.Debug().Str("object", work.FormatBucketPath()).Int64("written-bytes", work.Written).Send()

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

// At exit, due to the bug above, we visit every worker we have initialized and
// call Close to make sure the objects get committed. The nil check is the only
// way we can be sure not to close one twice
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
