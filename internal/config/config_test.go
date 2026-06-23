package config

import "testing"

func TestDefaultAppendsTestMixlrChannel(t *testing.T) {
	t.Setenv("TEST_MIXLR_CHANNEL", "orocash-music, defqon1blue")

	cfg := Default()

	if cfg.Channels[len(cfg.Channels)-1] != "orocash-music" {
		t.Fatalf("expected test channel appended last, got %q", cfg.Channels[len(cfg.Channels)-1])
	}

	count := 0
	for _, ch := range cfg.Channels {
		if ch == "defqon1blue" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected duplicate default channel skipped, got %d copies", count)
	}
}
