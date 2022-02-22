package main

import (
	"bytes"
	"context"
	"regexp"
	"testing"
	"time"
)

//
// FIXTURES
//
var work1 = NewObjectWorker(
	"sipiyou",
	"woopsie.example.com",
	"{{.InputTag}}/{{.Yyyy}}/{{.Mm}}/{{.Dd}}/{{.Timestamp}}",
	12345,
	1234,
	CompressionGzip,
)
var work2 = NewObjectWorker(
	"mermermy",
	"woopsie.example.com",
	"{{.IsoDateTime}}",
	12345,
	1234,
	CompressionNone,
)

////////////

func Test_objectNameData_String(t *testing.T) {
	ond := objectNameData{
		InputTag:    "hello",
		BeginTime:   time.Now(),
		Dd:          "17",
		IsoDateTime: "20220217T001600Z",
		Mm:          "02",
		Timestamp:   time.Now().Unix(),
		Yyyy:        "2022",
	}

	type args struct {
		ond *objectNameData
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "format #1",
			args: args{&ond},
			want: `&main.objectNameData{InputTag:"hello", BeginTime:time.Date\(\d{4}, time.[a-zA-Z]+, \d+, \d+, \d+, \d+, \d+, time.Local\), Dd:\"17\", IsoDateTime:\"20220217T001600Z\", Mm:\"02\", Timestamp:\d+, Yyyy:\"2022\"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.args.ond.String()
			rx := regexp.MustCompile(tt.want)
			if rx.FindStringIndex(got) == nil {
				t.Errorf("wanted: `%s` got: `%s`", tt.want, got)
			}
		})
	}
}

func Test_formatObjectName(t *testing.T) {
	work1.last = time.Now()
	work2.last = time.Now()
	tests := []struct {
		name   string
		worker *ObjectWorker
		want   string
	}{
		{name: "many parts", worker: work1, want: `sipiyou/\d{4}/\d\d/\d\d/\d+\.gz`},
		{name: "iso, none compression", worker: work2, want: `\d{4}\d{4}T\d{6}Z`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rx := regexp.MustCompile(tt.want)
			got := tt.worker.formatObjectName()
			if rx.FindStringIndex(got) == nil {
				t.Errorf("wanted: `%s` got: `%s`", tt.want, got)
			}
		})
	}
}

func Test_FormatBucketPath(t *testing.T) {
	work1.last = time.Now()
	work1.objectPath = work1.formatObjectName()
	work1.Writer = &storageWriter{}
	tests := []struct {
		name   string
		worker *ObjectWorker
		want   string
	}{
		{want: `gs://woopsie.example.com/sipiyou/\d{4}/\d\d/\d\d/\d+\.gz$`, worker: work1},
		{want: `\[closed\]`, worker: work2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rx := regexp.MustCompile(tt.want)
			if got := tt.worker.FormatBucketPath(); rx.FindStringIndex(got) == nil {
				t.Errorf("wanted: `%s` got: `%s`", tt.want, got)
			}
		})
	}
}

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

func newStorageClientForTest(ctx context.Context) (IStorageClient, error) {
	return &storageClientForTest{}, nil
}

func Test_beginStreaming(t *testing.T) {
	begin := time.Now()
	ctx := context.Background()
	cli, _ := newStorageClientForTest(ctx)

	work1.Written = 19
	work1.objectPath = "oh-no"

	work1.beginStreaming(cli)

	// these should be equal, rounded to the nearest minute
	minute, _ := time.ParseDuration("1m")
	diff := work1.last.Sub(begin).Round(minute)
	if diff != 0 {
		t.Error("beginStreaming() did not reset .last")
	}

	if work1.Written != 0 {
		t.Error("beginStreaming() did not reset .Written")
	}

	want := `sipiyou/\d{4}/\d\d/\d\d/\d+\.gz$`
	rx := regexp.MustCompile(want)
	if rx.FindStringIndex(work1.objectPath) == nil {
		t.Errorf("wanted: `%s` got: `%s`", want, work1.objectPath)
	}
}
