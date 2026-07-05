package stream

import "testing"

func drained(ch <-chan struct{}) int {
	n := 0
	for {
		select {
		case <-ch:
			n++
		default:
			return n
		}
	}
}

func TestHubNotifyWakesSubscriber(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe("sub1")
	defer cancel()

	h.Notify("sub1")
	if drained(ch) != 1 {
		t.Fatal("expected one wakeup after Notify")
	}
}

func TestHubNotifyCoalesces(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe("sub1")
	defer cancel()

	h.Notify("sub1")
	h.Notify("sub1")
	h.Notify("sub1")
	if got := drained(ch); got != 1 {
		t.Fatalf("wakeups should coalesce to 1, got %d", got)
	}
}

func TestHubIsolatesSubscriptions(t *testing.T) {
	h := NewHub()
	a, ca := h.Subscribe("A")
	defer ca()
	b, cb := h.Subscribe("B")
	defer cb()

	h.Notify("A")
	if drained(a) != 1 || drained(b) != 0 {
		t.Fatal("Notify must only wake the matching subscription")
	}
	h.Notify("nobody") // no subscribers → no panic, no effect
}

func TestHubFanOutToMultipleSubscribers(t *testing.T) {
	h := NewHub()
	c1, cancel1 := h.Subscribe("sub")
	defer cancel1()
	c2, cancel2 := h.Subscribe("sub")
	defer cancel2()

	h.Notify("sub")
	if drained(c1) != 1 || drained(c2) != 1 {
		t.Fatal("both subscribers of the same id should wake")
	}
}

func TestHubUnsubscribeStopsWakeups(t *testing.T) {
	h := NewHub()
	ch, cancel := h.Subscribe("sub")
	cancel()
	cancel() // idempotent — must not panic
	h.Notify("sub")
	if drained(ch) != 0 {
		t.Fatal("unsubscribed channel must not receive wakeups")
	}
}
