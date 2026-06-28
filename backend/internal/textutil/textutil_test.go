package textutil

import "testing"

func TestContentHashIdentical(t *testing.T) {
	h1 := ContentHash("Big News", "Body here.")
	h2 := ContentHash("Big News", "Body here.")
	if h1 != h2 {
		t.Fatal("identical content should hash identically")
	}
}

func TestContentHashIgnoresCaseAndPunctuation(t *testing.T) {
	if ContentHash("Big News!", "Body, here.") != ContentHash("big news", "body here") {
		t.Fatal("hash should ignore case and punctuation")
	}
}

func TestContentHashLongBodyTruncated(t *testing.T) {
	long := ""
	for i := 0; i < 6000; i++ {
		long += "a"
	}
	// 5000 'a' vs 6000 'a' normalize+truncate to the same 5000-char basis.
	if ContentHash("t", long) != ContentHash("t", long[:5000]) {
		t.Fatal("body should be truncated to 5000 chars")
	}
}

func TestJaccardIdenticalIsOne(t *testing.T) {
	a := TokenSetString("Magnitude earthquake hits Mindanao region")
	if got := Jaccard(a, a); got != 1 {
		t.Fatalf("want 1, got %v", got)
	}
}

func TestJaccardPartialOverlap(t *testing.T) {
	a := TokenSetString("Earthquake hits the Mindanao region today")
	b := TokenSetString("A strong earthquake struck Mindanao region")
	score := Jaccard(a, b)
	if !(score > 0.3 && score < 1) {
		t.Fatalf("expected partial overlap, got %v", score)
	}
}

func TestJaccardDisjointAndEmpty(t *testing.T) {
	if got := Jaccard(TokenSetString("apple banana"), TokenSetString("zebra yak")); got != 0 {
		t.Fatalf("disjoint want 0, got %v", got)
	}
	if got := Jaccard("", "anything here"); got != 0 {
		t.Fatalf("empty want 0, got %v", got)
	}
}

func TestStripHtml(t *testing.T) {
	if got := StripHtml("<p>Hello &amp; <b>world</b></p>"); got != "Hello & world" {
		t.Fatalf("got %q", got)
	}
	if got := StripHtml("<style>x{}</style><script>1</script><p>hi</p>"); got != "hi" {
		t.Fatalf("got %q", got)
	}
	if got := StripHtml(""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestStripHtmlEntities(t *testing.T) {
	got := StripHtml("a &lt;b&gt; c &#39;d&apos; &quot;e&quot;&nbsp;f")
	if got != `a <b> c 'd' "e" f` {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeText(t *testing.T) {
	if got := NormalizeText("Hello, WORLD!!  Foo-bar"); got != "hello world foo bar" {
		t.Fatalf("got %q", got)
	}
}

func TestTokenSetStringSortedNoStopwords(t *testing.T) {
	// stopwords (the) and short tokens (to) dropped; output sorted.
	if got := TokenSetString("The quick to fox"); got != "fox quick" {
		t.Fatalf("got %q", got)
	}
	if got := TokenSetString(""); got != "" {
		t.Fatalf("empty got %q", got)
	}
}

func TestFirstSentences(t *testing.T) {
	if got := FirstSentences("First one. Second two. Third three.", 2); got != "First one. Second two." {
		t.Fatalf("got %q", got)
	}
	if got := FirstSentences("no punctuation here", 1); got != "no punctuation here" {
		t.Fatalf("got %q", got)
	}
}

func TestFirstSentencesCapped(t *testing.T) {
	long := ""
	for i := 0; i < 600; i++ {
		long += "x"
	}
	long += "."
	if got := FirstSentences(long, 1); len(got) != 500 {
		t.Fatalf("want 500, got %d", len(got))
	}
}
