package render

import (
	"reflect"
	"testing"
)

func TestImg2WebPEncoderBuildsExpectedCommand(t *testing.T) {
	runner := &fakeRunner{}
	encoder := img2webpEncoder{Runner: runner, Binary: "img2webp"}
	err := encoder.Encode("out.webp", []frameSpec{{Path: "a.png", DurationMS: 125}, {Path: "b.png", DurationMS: 275}})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	call := runner.snapshotCalls()[0]
	want := []string{"img2webp", "-loop", "0", "-lossless", "-o", "out.webp", "-d", "125", "a.png", "-d", "275", "b.png"}
	if !reflect.DeepEqual(call, want) {
		t.Fatalf("call = %#v, want %#v", call, want)
	}
}
