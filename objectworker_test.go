package main

import (
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
	work2.last = time.Now()
	tests := []struct {
		name   string
		worker *ObjectWorker
		want   string
	}{
		{want: `gs://woopsie.example.com/sipiyou/\d{4}/\d\d/\d\d/\d+$`, worker: work1},
		{want: `gs://woopsie.example.com/sipiyou/\d{4}/\d\d/\d\d/\d+$`, worker: work2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rx := regexp.MustCompile(tt.want)
			if got := work1.FormatBucketPath(); rx.FindStringIndex(got) == nil {
				t.Errorf("wanted: `%s` got: `%s`", tt.want, got)
			}
		})
	}
}
