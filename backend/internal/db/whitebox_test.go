package db

import "testing"

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 7: "7", 123: "123"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Fatalf("itoa(%d)=%q want %q", in, got, want)
		}
	}
}

func TestDeref(t *testing.T) {
	if deref(nil) != "" {
		t.Fatal("nil → empty")
	}
	s := "x"
	if deref(&s) != "x" {
		t.Fatal("ptr → value")
	}
}
