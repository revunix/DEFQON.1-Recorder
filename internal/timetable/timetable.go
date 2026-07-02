package timetable

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/revunix/defqon1-recorder/internal/util"
)

type Set struct {
	Stage string
	DJ    string
	Start time.Time
	End   time.Time
}

type Timetable struct {
	sets []Set
}

// NewWithSets builds a timetable from an explicit set list (useful for tests).
func NewWithSets(sets []Set) *Timetable {
	return &Timetable{sets: sets}
}

func (t *Timetable) Size() int { return len(t.sets) }

func (t *Timetable) CurrentSet(stage string) *Set {
	now := time.Now()
	for i := range t.sets {
		s := &t.sets[i]
		if s.Stage == stage && !now.Before(s.Start) && !now.After(s.End) {
			return s
		}
	}
	return nil
}

// Upcoming returns the next future set for each stage, ordered by start time.
func (t *Timetable) Upcoming() []Set {
	now := time.Now()
	seen := make(map[string]bool)
	var result []Set
	for _, s := range t.sets {
		if s.Start.After(now) && !seen[s.Stage] {
			result = append(result, s)
			seen[s.Stage] = true
		}
	}
	return result
}

// SetsForStage returns every known set for a stage, ordered by start time. It
// is used by the splitter to cut a recording into per-set files.
func (t *Timetable) SetsForStage(stage string) []Set {
	var out []Set
	for _, s := range t.sets {
		if s.Stage == stage {
			out = append(out, s)
		}
	}
	return out
}

type rawStage struct {
	Stage string  `json:"stage"`
	Sets  [][]any `json:"sets"`
}

func Load(path string) (*Timetable, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw []rawStage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	loc := util.Berlin()
	var all []Set

	for _, stage := range raw {
		// Parse every entry, including time-only markers (no DJ name). A marker
		// such as [2026, 6, 26, 23, 0] is kept only to bound the end of the
		// previous set; it is not itself a playable set, so it must not become a
		// "TBA" entry that spans the overnight gap until the next day.
		stageSets := make([]Set, 0, len(stage.Sets))
		for _, rawSet := range stage.Sets {
			if len(rawSet) < 5 {
				continue
			}
			start := time.Date(
				util.ToInt(rawSet[0]),
				time.Month(util.ToInt(rawSet[1])),
				util.ToInt(rawSet[2]),
				util.ToInt(rawSet[3]),
				util.ToInt(rawSet[4]),
				0, 0, loc,
			)
			dj := ""
			if len(rawSet) > 5 {
				parts := make([]string, 0, len(rawSet)-5)
				for _, p := range rawSet[5:] {
					if s, ok := p.(string); ok {
						parts = append(parts, s)
					}
				}
				dj = strings.TrimSpace(strings.Join(parts, " "))
				if dj == "" {
					dj = "TBA"
				}
			}
			stageSets = append(stageSets, Set{Stage: stage.Stage, DJ: dj, Start: start})
		}

		sort.SliceStable(stageSets, func(i, j int) bool {
			return stageSets[i].Start.Before(stageSets[j].Start)
		})

		for i := range stageSets {
			end := stageSets[i].Start.Add(60 * time.Minute)
			if i < len(stageSets)-1 {
				end = stageSets[i+1].Start
			}
			stageSets[i].End = end
			if stageSets[i].DJ == "" {
				continue // marker: end boundary only, not a playable set
			}
			all = append(all, stageSets[i])
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Start.Before(all[j].Start)
	})

	return &Timetable{sets: all}, nil
}
