package util

import (
	"fmt"
	"math"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	// Embed IANA tzdata for Windows and minimal container builds, which may not
	// have a system zoneinfo database to satisfy time.LoadLocation.
	_ "time/tzdata"
)

var (
	berlinOnce sync.Once
	berlinLoc  *time.Location
)

func Berlin() *time.Location {
	berlinOnce.Do(func() {
		loc, err := time.LoadLocation("Europe/Berlin")
		if err != nil {
			loc = time.FixedZone("CET", 3600)
		}
		berlinLoc = loc
	})
	return berlinLoc
}

func Sanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0x20 && r <= 0x7E {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Alnum keeps only ASCII letters and digits. Useful for building scene-style
// file-name segments where no separators or special characters are allowed
// inside a field.
func Alnum(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// AlnumDash keeps ASCII letters, digits and hyphens. It is used for DJ/artist
// segments in scene-style file names where a hyphen is meaningful (e.g.
// "D-Sturb"), unlike Alnum which would strip it.
func AlnumDash(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SystemUser returns the current OS user name, stripped of any Windows domain
// prefix (e.g. "DOMAIN\\user" -> "user") and reduced to alphanumerics. It falls
// back to "anonymous" when the user cannot be determined.
func SystemUser() string {
	u, err := user.Current()
	if err != nil || u.Username == "" {
		return "anonymous"
	}
	name := u.Username
	if i := strings.LastIndex(name, `\`); i >= 0 {
		name = name[i+1:]
	}
	name = Alnum(name)
	if name == "" {
		return "anonymous"
	}
	return name
}

func FormatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 Bytes"
	}
	k := 1024.0
	sizes := []string{"Bytes", "KB", "MB", "GB", "TB"}
	i := int(math.Floor(math.Log(float64(bytes)) / math.Log(k)))
	if i >= len(sizes) {
		i = len(sizes) - 1
	}
	val := float64(bytes) / math.Pow(k, float64(i))
	return fmt.Sprintf("%.2f %s", val, sizes[i])
}

func FormatDuration(totalMinutes int) string {
	if totalMinutes < 1 {
		return "Live"
	}
	days := totalMinutes / 1440
	hours := (totalMinutes % 1440) / 60
	minutes := totalMinutes % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if days == 0 && minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	return strings.Join(parts, " ")
}

func ToInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

func FormatThousands(n int) string {
	s := strconv.Itoa(abs(n))
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
