package main

import (
	"C"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	gcsClient IStorageClient

	// string to uniquely identify this output plugin instance
	outputID string

	// a template for the object filename that gets created in the bucket. this uses golang text/template syntax.
	// The following placeholders are recognized:
	// {{ .InputTag }} the tag of the associated fluent "input" being flushed, e.g. "cpu"
	// {{ .Timestamp }} timestamp using unix seconds since 1970-01-01
	// {{ .IsoDateTime }} 14-digit YYYYmmddTHHMMSSZ datetime format, UTC
	// {{ .Yyyy }} {{ .Mm }} {{ .Dd }} year, month, day
	// {{ .Uuid }} a random UUID
	// {{ .BeginTime.Format "2006...." }} .beginTime is a time.Time() object and you can use any method on it;
	// 								      for example, you can call .Format() as shown and get any format you want
	// The object created will be in gs://BUCKET/
	// default "{{ .InputTag }}-{{ .Timestamp }}-{{ .Uuid }}"
	objectNameTemplate string

	// internal-use; map of inputTag to a gcs api client worker
	workers map[string](*ObjectWorker)
}

// gzip or none
type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionGzip CompressionType = "gzip"
)

const (
	FB_OUTPUT_NAME = "gcs"
)

var (
	VERSION   string                    // to set this, build with --ldflags="-X main.VERSION=vx.y.z"
	instances map[string](*outputState) = make(map[string](*outputState))
)

// global access to the fluent-bit API through this object
var flbAPI IFLBOutputAPI = &flbOutputAPIWrapper{}

// global access to the gcp storage API through this object
var storageAPI IStorageAPI = &storageAPIWrapper{}

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

// get a string value from the config, enforcing that it is set
func getConfigStrRequired(plugin unsafe.Pointer, skey string) string {
	var val string
	if val = flbAPI.FLBPluginConfigKey(plugin, skey); val == "" {
		flbAPI.FLBPluginUnregister(plugin)
		logger.Fatal().Str("field", "Bucket").Msgf("required field %s is missing from 1 or more [output] blocks. Check your .conf and add this field.", skey)
	}
	return val
}

// get a string value from the config, substituting the default if blank
func getConfigStrDefault(plugin unsafe.Pointer, skey, dfl string) string {
	var val string
	if val = flbAPI.FLBPluginConfigKey(plugin, skey); val == "" {
		val = dfl
	}
	return val
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	// change the logging style for dev testing
	if dev := os.Getenv("OUT_GCS_DEV_LOGGING"); dev != "" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		logger.Info().Str("OUT_GCS_DEV_LOGGING", dev).Msg("Enabling dev-style logging")
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// [OUTPUT] sections for the gcs plugin must have these fields
	bucket := getConfigStrRequired(plugin, "Bucket")
	outputID := getConfigStrRequired(plugin, "OutputID")

	objectNameTemplate := getConfigStrDefault(plugin, "ObjectNameTemplate", "{{ .InputTag }}-{{ .Timestamp }}")

	// create a GCS API client for this output instance, or die
	gcsctx := context.Background()
	client, err := storageAPI.NewClient(gcsctx)
	if err != nil {
		flbAPI.FLBPluginUnregister(plugin)
		logger.Fatal().Msgf("FLBPluginInit() NewStorageClient() %s", err.Error())
		return output.FLB_ERROR
	}

	// parse configuration for this output instance
	ost := outputState{
		bucket:               bucket,
		bufferSizeKiB:        5000,
		bufferTimeoutSeconds: 300,
		compression:          CompressionNone,
		gcsClient:            client,
		outputID:             outputID,
		objectNameTemplate:   objectNameTemplate,

		// initialize workers; this instance will eventually add 1 worker per input to this map
		workers: map[string]*ObjectWorker{},
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
func FLBPluginFlushCtx(plugin, data unsafe.Pointer, length C.int, tag *C.char) int {
	//notest
	state := flbAPI.FLBPluginGetContext(plugin).(outputState)
	return flbPluginFlushCtxGo(&state, data, int(length), C.GoString(tag))
}

// logs are emitted as 2-arrays of [timestamp, fields{}]
type logRec []interface{}

// fields in a log record have string keys and values are mostly strings but may be something else
type logFields map[string]interface{}

// higher-level flush implementation accepting parameters which are mostly gotypes instead of Ctypes
func flbPluginFlushCtxGo(state *outputState, data unsafe.Pointer, length int, tagName string) int {
	work, exists := state.workers[tagName]
	if !exists {
		work = NewObjectWorker(
			tagName,
			state.bucket,
			state.objectNameTemplate,
			state.bufferSizeKiB,
			state.bufferTimeoutSeconds,
			state.compression,
		)
		state.workers[tagName] = work
	}

	dec := flbAPI.NewDecoder(data, length)
	buf := new(bytes.Buffer)

	// Gets called with a batch of records to be written to an instance.
	// Decode each rec
	for {
		rc, ts, rec := flbAPI.GetRecord(dec)
		if rc != 0 {
			break
		}
		buf.WriteString(fmt.Sprintf("%s: ", tagName))

		timestamp := float64((ts.(output.FLBTime)).UnixMicro())
		fields := logFields{}
		go_rec := logRec{timestamp / 1e6, fields}

		for key, val := range rec {
			key := key.(string)

			switch val := val.(type) {
			case []byte:
				fields[key] = string(val)
			default:
				fields[key] = val
			}
		}
		marshalled, _ := json.Marshal(go_rec)
		buf.Write(marshalled)
		buf.WriteString("\n")
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
		logger.Debug().Str("outputID", inst.outputID).Msgf("cleaning up instance %s", inst.outputID)
		for _, worker := range inst.workers {
			// due to the FLBPluginExitCtx bug (see comment above), we just have
			// to check and see whether each one is closed here.
			if worker.Writer != nil {
				worker.Commit()
			}
		}
	}
	return output.FLB_OK
}

// utility function for converting byte arrays
func goBytesToCBytes(data []byte) unsafe.Pointer {
	return unsafe.Pointer(C.CBytes(data))
}

func main() {} //notest
