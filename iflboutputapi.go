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
