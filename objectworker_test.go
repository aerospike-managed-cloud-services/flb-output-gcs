package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"io/ioutil"
	"regexp"
	"testing"
	"time"
)

//
// FIXTURES
//
func newWork1() *ObjectWorker {
	return NewObjectWorker(
		"sipiyou",
		"woopsie.example.com",
		"{{.InputTag}}/{{.Yyyy}}/{{.Mm}}/{{.Dd}}/{{.Timestamp}}",
		12345,
		1234,
		CompressionGzip,
	)
}

func newWork2() *ObjectWorker {
	return NewObjectWorker("mermermy",
		"woopsie.example.com",
		"{{.IsoDateTime}}",
		12345,
		1234,
		CompressionNone,
	)

}

// no actual state is held here, so we'll just make one of these globally
var sapi = &storageAPIForTest{}

////////////

// do we pretty-print an objectNameData{}?
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

// do we create a formatted path out of our objectNameData{} and template?
// do we attach the .gz extension when appropriate?
func Test_formatObjectName(t *testing.T) {
	work1 := newWork1()
	work2 := newWork2()
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

// do we create a full gs:// url out of our worker, template, and compression flag?
// do we present a closed path when the worker is not in a writeable state?
func Test_FormatBucketPath(t *testing.T) {
	work1 := newWork1()
	work2 := newWork2()
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

// do we set up and tear down the bucket writing objects when asked to begin streaming?
func Test_beginStreaming(t *testing.T) {
	begin := time.Now()
	ctx := context.Background()
	cli, _ := sapi.NewClient(ctx)

	work1 := newWork1()
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

	if work1.Writer == nil {
		t.Error("Writer was nil before worker was committed")
	}

	work1.Commit()

	if work1.Writer != nil {
		t.Error("Writer is dangling on a worker after commit")
	}

	// If Stop() returns true, the timer was never previously stopped.
	// Should return false, indicating the timer was already stopped by calling Commit()
	if work1.timer.Stop() {
		t.Error("The timer was not stopped by calling Commit()")
	}
}

// ObjectWorker.Put() with a gzip stream, do we produce the write stream of bytes?
func Test_Put_gzip(t *testing.T) {
	ctx := context.Background()
	cli, _ := sapi.NewClient(ctx)

	buf := bytes.NewBufferString("abz")
	work1 := newWork1()

	work1.Put(cli, *buf)

	wri := work1.Writer.(*storageWriterForTest)
	zreader, _ := gzip.NewReader(wri.buf)
	defer zreader.Close()
	if bb, _ := ioutil.ReadAll(zreader); !bytes.Equal(bb, []byte{'a', 'b', 'z'}) {
		t.Errorf("Put() failed, buffer written was '%s'", bb)
	}
}

// ObjectWorker.Put() with uncompressed stream, and we exceed the byte limit, do we commit automatically?
func Test_Put_plain_commit(t *testing.T) {
	ctx := context.Background()
	cli, _ := sapi.NewClient(ctx)

	buf := bytes.NewBufferString("abc")
	work2 := newWork2()
	work2.bytesMax = 2

	work2.beginStreaming(cli)
	wri := work2.Writer.(*storageWriterForTest)
	work2.Put(cli, *buf)

	if work2.Writer != nil {
		t.Errorf("work2.Writer should have been closed after write of 3 bytes, but was %#v", work2.Writer)
	}

	if wri.buf.String() != "abc" {
		t.Errorf("Put() failed, buffer written was '%s'", wri.buf.String())
	}
}

// do we commit automatically when a timer expires?
func Test_timerExpired(t *testing.T) {
	work2 := newWork2()
	work2.bufferTimeoutMicro = 3

	ctx := context.Background()
	sapi := &storageAPIForTest{}
	cli, _ := sapi.NewClient(ctx)
	work2.beginStreaming(cli)

	if work2.Writer == nil {
		t.Error("work2.Writer was nil after calling beginStreaming, something's up")
	}

	t.Log("sleeping for 1ms to force the timer (0.003ms) to expire")
	time.Sleep(1 * time.Millisecond)

	if work2.Writer != nil {
		t.Error("work2.Writer was left dangling after the timer expiration should have caused a commit")
	}

}
