package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIBaseURL           string
	Channels             []string
	RecordingsDir        string
	TimetablePath        string
	ToolsDir             string
	CheckInterval        time.Duration
	StalledCheckInterval time.Duration
	StalledTimeout       time.Duration
	TUIUpdateInterval    time.Duration
}

func Default() Config {
	channels := []string{
		"defqon-1-magenta", // DQ.1 Magenta
		"defqon1purple",    // DQ.1 Purple
		"defqon1white",     // DQ.1 White
		"defqon-1-brown",   // DQ.1 Brown
		"defqon1pink",      // DQ.1 Pink
		"defqon1blue",      // DQ.1 Blue
		"defqon1indigo",    // DQ.1 Indigo
		"defqon1yellow",    // DQ.1 Yellow
		"defqon1orange",    // DQ.1 Orange
		"defqon1silver",    // DQ.1 Silver
		"defqon1green",     // DQ.1 Green
		"defqon1gold",      // DQ.1 Gold
		"defqon1black",     // DQ.1 Black
		"defqon1uv",        // DQ.1 UV
	}
	channels = appendEnvChannels(channels, "TEST_MIXLR_CHANNEL")

	return Config{
		APIBaseURL:           "https://apicdn.mixlr.com/v3/channel_view/",
		Channels:             channels,
		RecordingsDir:        envOr("RECORDINGS_DIR", "recordings"),
		TimetablePath:        envOr("TIMETABLE_PATH", "dq-timetable.json"),
		ToolsDir:             envOr("TOOLS_DIR", exeDir()),
		CheckInterval:        envDurationMSOr("CHECK_INTERVAL_MS", 60_000*time.Millisecond),
		StalledCheckInterval: 30 * time.Second,
		StalledTimeout:       60 * time.Second,
		TUIUpdateInterval:    envDurationMSOr("TUI_UPDATE_INTERVAL_MS", 2_000*time.Millisecond),
	}
}

func appendEnvChannels(channels []string, key string) []string {
	for _, raw := range strings.Split(os.Getenv(key), ",") {
		channel := strings.TrimSpace(raw)
		if channel == "" || contains(channels, channel) {
			continue
		}
		channels = append(channels, channel)
	}
	return channels
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// exeDir returns the directory of the running executable, used to locate
// bundled yt-dlp / ffmpeg binaries.
func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

func envDurationMSOr(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	ms, err := strconv.Atoi(v)
	if err != nil || ms <= 0 {
		return def
	}
	return time.Duration(ms) * time.Millisecond
}
