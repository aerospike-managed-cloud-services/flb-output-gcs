package main

import (
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

func Test_FLBPluginInit(t *testing.T) {
	plugin := unsafe.Pointer(&outputPluginForTest{})
	config_good := flbOutputAPIForTest{config: opcConfig{
		"BufferSizeKiB":        "19",
		"BufferTimeoutSeconds": "300",
		"Compression":          "",
		"Bucket":               "bucketymcbucketface.example.com",
		"OutputID":             "xyz",
		"ObjectNameTemplate":   "",
	}}

	type args struct {
		myAPI *flbOutputAPIForTest
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{name: "basic bs",
			args: args{&config_good},
			want: 19,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageAPI = &storageAPIForTest{}
			flbAPI = tt.args.myAPI
			FLBPluginInit(plugin)
			outConfig := flbAPI.FLBPluginGetContext(plugin).(outputState)
			expected := outputState{
				bucket:               "bucketymcbucketface.example.com",
				bufferSizeKiB:        19,
				bufferTimeoutSeconds: 300,
				compression:          CompressionNone,
				gcsClient:            outConfig.gcsClient,
				outputID:             "xyz",
				objectNameTemplate:   "{{ .InputTag }}-{{ .Timestamp }}",
				workers:              map[string]*ObjectWorker{},
			}
			if !reflect.DeepEqual(outConfig, expected) {
				t.Errorf("outConfig = %#v did not match expected %#v", outConfig, expected)
			}
		})
	}
}
