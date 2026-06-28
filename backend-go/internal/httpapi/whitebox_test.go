package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestWriteJSONMarshalError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, 200, make(chan int)) // channels can't be marshaled
	if rec.Code != 500 {
		t.Fatalf("marshal error should yield 500, got %d", rec.Code)
	}
}

func TestJsonRawError(t *testing.T) {
	if jsonRaw(make(chan int)) != nil {
		t.Fatal("unmarshalable value should yield nil RawJSON")
	}
}

func TestTimePtrNil(t *testing.T) {
	if timePtr(nil) != nil {
		t.Fatal("nil PrismaTime should map to nil")
	}
}
