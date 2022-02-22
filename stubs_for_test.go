// stub implementations of google storage and flb-output APIs so tests can use them

package main

import (
	"bytes"
	"context"
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
)

// google storage

type storageWriterForTest struct {
	buf *bytes.Buffer
}

func (sto *storageWriterForTest) Close() error {
	return nil
}

func (sto *storageWriterForTest) Write(b []byte) (n int, err error) {
	return sto.buf.Write(b)
}

func (sto *storageWriterForTest) SetChunkSize(n int) {
}

type storageClientForTest struct {
}

func (sto *storageClientForTest) NewWriterFromBucketObjectPath(bucket, path string, ctx context.Context) IStorageWriter {
	return &storageWriterForTest{}
}

type storageAPIForTest struct{}

func (sapi *storageAPIForTest) NewClient(ctx context.Context) (IStorageClient, error) {
	return &storageClientForTest{}, nil
}

// fluent-bit output api

type outputPluginForTest struct{}

type flbOutputAPIForTest struct {
	config map[string]string
	ctx    interface{}
}

func (opc *flbOutputAPIForTest) FLBPluginConfigKey(plugin unsafe.Pointer, skey string) string {
	return opc.config[skey]
}

var registration map[string]string = make(map[string]string)

func (opc *flbOutputAPIForTest) FLBPluginRegister(plugin unsafe.Pointer, name string, desc string) int {
	registration["name"] = name
	registration["desc"] = desc
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
