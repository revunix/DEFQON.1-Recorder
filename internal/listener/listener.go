package listener

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"

	"github.com/revunix/defqon1-recorder/internal/tools"
)

const (
	sampleRate   = 48000
	channelCount = 2
)

type State string

const (
	StateStopped  State = "Stopped"
	StateStarting State = "Starting"
	StatePlaying  State = "Playing"
)

type Snapshot struct {
	State State
	Stage string
}

type Player struct {
	ffmpeg string

	mu      sync.Mutex
	ctx     *oto.Context
	current *session
}

type session struct {
	stage  string
	cmd    *exec.Cmd
	player *oto.Player
	done   chan struct{}
	state  State
}

func New(toolsDir string) *Player {
	return &Player{ffmpeg: tools.Resolve(toolsDir).FFmpeg}
}

func (p *Player) Play(stage, streamURL string) error {
	if err := validateURL(streamURL); err != nil {
		return err
	}

	p.Stop()

	ctx, err := p.context()
	if err != nil {
		return err
	}

	args := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-i", streamURL,
		"-vn",
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-ac", fmt.Sprint(channelCount),
		"-ar", fmt.Sprint(sampleRate),
		"pipe:1",
	}
	cmd := exec.Command(p.ffmpeg, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	otoPlayer := ctx.NewPlayer(stdout)
	otoPlayer.SetBufferSize(sampleRate * channelCount * 2)

	s := &session{
		stage:  stage,
		cmd:    cmd,
		player: otoPlayer,
		done:   make(chan struct{}),
		state:  StateStarting,
	}

	p.mu.Lock()
	p.current = s
	p.mu.Unlock()

	go drain(stderr)
	otoPlayer.Play()
	p.setState(s, StatePlaying)

	go p.wait(s)
	return nil
}

func (p *Player) Stop() {
	p.mu.Lock()
	s := p.current
	if s == nil {
		p.mu.Unlock()
		return
	}
	p.current = nil
	p.mu.Unlock()

	s.stop()
}

func (p *Player) Snapshot() Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return Snapshot{State: StateStopped}
	}
	return Snapshot{State: p.current.state, Stage: p.current.stage}
}

func (p *Player) context() (*oto.Context, error) {
	p.mu.Lock()
	if p.ctx != nil {
		ctx := p.ctx
		p.mu.Unlock()
		return ctx, nil
	}

	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		Format:       oto.FormatSignedInt16LE,
		BufferSize:   500 * time.Millisecond,
	})
	if err != nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("audio device: %w", err)
	}

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		p.mu.Unlock()
		return nil, errors.New("audio device did not become ready")
	}

	p.ctx = ctx
	p.mu.Unlock()

	return ctx, nil
}

func (p *Player) setState(s *session, state State) {
	p.mu.Lock()
	if p.current == s {
		s.state = state
	}
	p.mu.Unlock()
}

func (p *Player) wait(s *session) {
	_ = s.cmd.Wait()
	_ = s.player.Close()
	close(s.done)

	p.mu.Lock()
	if p.current == s {
		p.current = nil
	}
	p.mu.Unlock()
}

func (s *session) stop() {
	if s.player != nil {
		_ = s.player.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
	}
}

func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}
	return nil
}

func drain(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
	}
}
