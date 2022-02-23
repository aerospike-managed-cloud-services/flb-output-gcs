package main

import (
	"context"
	"reflect"
	"testing"
	"unsafe"
)

type opcConfig map[string]string

// do we capture and correctly set name and desc during reg
func Test_FLBPluginRegister(t *testing.T) {
	plugin := unsafe.Pointer(&outputPluginForTest{})
	flbAPI = &flbOutputAPIForTest{config: opcConfig{}}

	VERSION = "v1.hello"

	for k := range registration {
		delete(registration, k)
	}

	rc := FLBPluginRegister(plugin)

	if (rc != 0) || (registration["name"] != FB_OUTPUT_NAME) || (registration["desc"] != "GCS bucket output v1.hello") {
		t.Errorf(`registration didn't work, %#v`, registration)
	}
}

// do we correctly convert (or fallback to default) for number, blank, and not-number
func Test_pluginConfigValueToInt(t *testing.T) {
	plugin := unsafe.Pointer(&outputPluginForTest{})
	config_has_key := flbOutputAPIForTest{config: opcConfig{"some_key": "19"}}
	config_blank_key := flbOutputAPIForTest{config: opcConfig{"some_key": ""}}
	config_bad_key := flbOutputAPIForTest{config: opcConfig{"some_key": "nineteen"}}

	type args struct {
		flbAPI *flbOutputAPIForTest
		skey   string
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{name: "int value",
			args: args{&config_has_key, "some_key"},
			want: 19,
		},
		{name: "blank value",
			args: args{&config_blank_key, "some_key"},
			want: 17,
		},
		{name: "unparseable value",
			args: args{&config_bad_key, "some_key"},
			want: 17,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int64

			// replace api with our test implementation
			flbAPI = tt.args.flbAPI

			if ret, ok := pluginConfigValueToInt(plugin, tt.args.skey); ok {
				result = ret
			} else {
				result = 17
			}

			if result != tt.want {
				t.Errorf(`pluginConfigValueToInt(%v, %s) ! match %d`, plugin, tt.args.skey, tt.want)
			}
		})
	}
}

// do we convert the text config into a working configured outputState;
// can we also do that twice, and then clean up and shut down both?
func Test_FLBPluginInit_Exit(t *testing.T) {
	// swap production storageAPI with a stub to prevent actual access
	// to gcp during this test
	storageAPI = &storageAPIForTest{}

	// build a "plugin" that just holds our config strings
	plugin1 := unsafe.Pointer(&outputPluginForTest{})
	config1 := flbOutputAPIForTest{config: opcConfig{
		"BufferSizeKiB":        "19",
		"BufferTimeoutSeconds": "300",
		"Compression":          "",
		"Bucket":               "bucketymcbucketface.example.com",
		"OutputID":             "1",
		"ObjectNameTemplate":   "",
	}}

	// set production flbAPI to this stub since there is no real fluent-bit
	// process that owns this thread
	flbAPI = &config1

	// init #1
	FLBPluginInit(plugin1)

	// make assertions about the config conversion that must have occurred
	outConfig1 := flbAPI.FLBPluginGetContext(plugin1).(outputState)
	expected := outputState{
		bucket:               "bucketymcbucketface.example.com",
		bufferSizeKiB:        19,
		bufferTimeoutSeconds: 300,
		compression:          CompressionNone,
		gcsClient:            outConfig1.gcsClient,
		outputID:             "1",
		objectNameTemplate:   "{{ .InputTag }}-{{ .Timestamp }}",
		workers:              map[string]*ObjectWorker{},
	}
	if !reflect.DeepEqual(outConfig1, expected) {
		t.Errorf("outConfig = %#v did not match expected %#v", outConfig1, expected)
	}

	//
	// continue: perform another init, and then call FLBPluginExit() to ensure cleanup succeeds
	//
	// build a "plugin" that just holds our config strings
	plugin2 := unsafe.Pointer(&outputPluginForTest{})
	config2 := flbOutputAPIForTest{config: opcConfig{
		"BufferSizeKiB":        "19",
		"BufferTimeoutSeconds": "300",
		"Compression":          "",
		"Bucket":               "bucketymcbucketface.example.com",
		"OutputID":             "2",
		"ObjectNameTemplate":   "",
	}}

	// This is ~cheating~; we replace the global flbAPI again.
	// At the moment this is ok due to the implementation detail that we don't
	// reference this global during Exit
	flbAPI = &config2

	// init #2
	FLBPluginInit(plugin2)

	// let's also beginStreaming on both instances so we have something to clean up
	ctx := context.Background()
	cli, _ := storageAPI.NewClient(ctx)

	work1 := NewObjectWorker(
		"1",
		"bucketymcbucketface.example.com",
		"2-{{.Timestamp}}",
		19,
		19,
		CompressionNone,
	)
	outConfig1.workers["1"] = work1
	work1.beginStreaming(cli)

	work2 := NewObjectWorker(
		"2",
		"bucketymcbucketface.example.com",
		"2-{{.Timestamp}}",
		19,
		19,
		CompressionNone,
	)
	outConfig2 := flbAPI.FLBPluginGetContext(plugin2).(outputState)
	outConfig2.workers["2"] = work2
	work2.beginStreaming(cli)

	// now start cleaning these up
	FLBPluginExit()

	for _, inst := range instances {
		for _, worker := range inst.workers {
			if worker.Writer != nil {
				t.Errorf("%s/%s .Writer was not cleaned up during Exit", inst.outputID, worker.formatObjectName())
			}
		}
	}
}
