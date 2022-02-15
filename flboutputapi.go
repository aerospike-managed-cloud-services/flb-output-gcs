// wraps github.com/fluent/fluent-bit-go/output to make it easier to unit test
package main

import (
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
)

type IFLBOutputAPI interface {
	FLBPluginConfigKey(plugin unsafe.Pointer, skey string) string
	FLBPluginRegister(plugin unsafe.Pointer, name string, desc string) int
	FLBPluginUnregister(plugin unsafe.Pointer)
	FLBPluginSetContext(plugin unsafe.Pointer, ctx interface{})
	FLBPluginGetContext(proxyContext unsafe.Pointer) interface{}
	NewDecoder(data unsafe.Pointer, length int) *output.FLBDecoder
	GetRecord(dec *output.FLBDecoder) (ret int, ts interface{}, rec map[interface{}]interface{})
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
