package main

import (
	"testing"
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
)

type flbOutputAPIForTest struct {
	config map[string]string
	ctx    interface{}
}

type outputPluginForTest struct{}

func (opc *flbOutputAPIForTest) FLBPluginConfigKey(plugin unsafe.Pointer, skey string) string {
	return opc.config[skey]
}

func (opc *flbOutputAPIForTest) FLBPluginRegister(plugin unsafe.Pointer, name string, desc string) int {
	return 0
}

func (opc *flbOutputAPIForTest) FLBPluginSetContext(plugin unsafe.Pointer, ctx interface{}) {
	opc.ctx = ctx
}

func (opc *flbOutputAPIForTest) FLBPluginGetContext(proxyContext unsafe.Pointer) interface{} {
	return opc.ctx
}

func (opc *flbOutputAPIForTest) FLBPluginUnregister(plugin unsafe.Pointer) {}

func (opc *flbOutputAPIForTest) NewDecoder(data unsafe.Pointer, length int) *output.FLBDecoder {
	return output.NewDecoder(data, length)
}

func (opc *flbOutputAPIForTest) GetRecord(dec *output.FLBDecoder) (int, interface{}, map[interface{}]interface{}) {
	return output.GetRecord(dec)
}

type opcConfig map[string]string

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
