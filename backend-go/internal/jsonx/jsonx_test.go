package jsonx

import "testing"

func TestMarshalNoHTMLEscape(t *testing.T) {
	b, err := Marshal(map[string]string{"k": "a & b < c > d"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"k":"a & b < c > d"}`
	if string(b) != want {
		t.Fatalf("got %s want %s", b, want)
	}
}

func TestMarshalNoTrailingNewline(t *testing.T) {
	b, _ := Marshal(123)
	if string(b) != "123" {
		t.Fatalf("got %q", b)
	}
}

func TestMarshalError(t *testing.T) {
	if _, err := Marshal(make(chan int)); err == nil {
		t.Fatal("expected error for unmarshalable type")
	}
}
