package status

import "testing"

func TestRegistryOrderAndSet(t *testing.T) {
	r := New([]string{"a", "b", "c"})

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(all))
	}
	if all[0].Online {
		t.Fatal("entries should start offline")
	}

	r.Set("b", "BLUE", true, 42, "https://listen.mixlr.com/live")
	r.Set("unknown", "X", true, 1, "https://listen.mixlr.com/unknown")

	all = r.All()
	// original order preserved, unknown appended last
	wantOrder := []string{"a", "b", "c", "unknown"}
	if len(all) != len(wantOrder) {
		t.Fatalf("expected %d entries, got %d", len(wantOrder), len(all))
	}
	for i, c := range wantOrder {
		if all[i].Channel != c {
			t.Errorf("index %d: channel = %q, want %q", i, all[i].Channel, c)
		}
	}

	var blue *Stream
	for i := range all {
		if all[i].Channel == "b" {
			blue = &all[i]
		}
	}
	if blue == nil || !blue.Online || blue.ListenerCount != 42 || blue.Stage != "BLUE" || blue.StreamURL == "" {
		t.Fatalf("blue stream not updated correctly: %+v", blue)
	}

	r.Set("b", "BLUE", false, 0, "https://listen.mixlr.com/live")
	all = r.All()
	for i := range all {
		if all[i].Channel == "b" && all[i].StreamURL != "" {
			t.Fatalf("offline stream should not keep listen URL: %+v", all[i])
		}
	}
}
