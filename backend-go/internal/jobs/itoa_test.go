package jobs

import "testing"

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 5: "5", 42: "42", -7: "-7"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Fatalf("itoa(%d)=%q want %q", in, got, want)
		}
	}
}
