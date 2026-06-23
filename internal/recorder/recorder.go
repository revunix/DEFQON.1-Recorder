package recorder

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/revunix/defqon1-recorder/internal/logging"
	"github.com/revunix/defqon1-recorder/internal/tools"
)

// mp3Bitrate is the constant bitrate used for the live MP3 transcoding.
const mp3Bitrate = "192k"

type Snapshot struct {
	Stage     string
	Path      string
	Listeners int
}

type recording struct {
	stage     string
	ytdlp     *exec.Cmd
	ffmpeg    *exec.Cmd
	path      string
	fileName  string
	listeners int
	lastSize  int64
	lastCheck time.Time
	mu        sync.Mutex
}

type Manager struct {
	dir          string
	stalledAfter time.Duration
	paths        tools.Paths
	log          logging.Logger

	mu     sync.RWMutex
	active map[string]*recording
	wg     sync.WaitGroup

	stopped int32
}

func New(dir string, stalledAfter time.Duration, toolsDir string, log logging.Logger) *Manager {
	if log == nil {
		log = logging.Noop{}
	}
	paths := tools.Resolve(toolsDir)
	log.Info(fmt.Sprintf("yt-dlp: %s | ffmpeg: %s", paths.YtDLP, paths.FFmpeg))
	return &Manager{
		dir:          dir,
		stalledAfter: stalledAfter,
		paths:        paths,
		log:          log,
		active:       make(map[string]*recording),
	}
}

func (m *Manager) IsRecording(stage string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.active[stage]
	return ok
}

func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.active)
}

func (m *Manager) Start(stage, streamURL string, listeners int) {
	fileName := fmt.Sprintf("%s_%s.mp3", stage, time.Now().UTC().Format("2006-01-02T15-04-05.000Z"))
	outputPath := filepath.Join(m.dir, fileName)

	// yt-dlp downloads the raw audio stream to stdout; ffmpeg transcodes it to
	// MP3 in real time. Transcoding live (instead of as a yt-dlp post-processor)
	// guarantees a genuine MP3 even when the stream is interrupted or killed,
	// because the conversion no longer depends on a clean download completion.
	ytdlpArgs := []string{"--no-part", "-f", "bestaudio", "--live-from-start"}
	if m.paths.FFmpegDir != "" {
		ytdlpArgs = append(ytdlpArgs, "--ffmpeg-location", m.paths.FFmpegDir)
	}
	ytdlpArgs = append(ytdlpArgs, "-o", "-", streamURL)

	// ffmpeg reads the raw stream from stdin, transcodes to MP3 and writes the
	// MP3 to its own stdout. Writing the output to a pipe (instead of a file)
	// defeats ffmpeg's internal file buffering: combined with -fflags
	// +flush_packets, ffmpeg hands off each packet promptly, and Go writes every
	// chunk straight to disk so the file size grows live.
	ffmpegArgs := []string{
		"-hide_banner", "-nostdin", "-loglevel", "error",
		"-i", "pipe:0",
		"-fflags", "+flush_packets",
		"-c:a", "libmp3lame", "-b:a", mp3Bitrate,
		"-f", "mp3", "pipe:1",
	}

	ytdlp := exec.Command(m.paths.YtDLP, ytdlpArgs...)
	ffmpeg := exec.Command(m.paths.FFmpeg, ffmpegArgs...)

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		m.log.Error(fmt.Sprintf("[%s] Pipe error: %s", stage, err))
		return
	}
	ytdlp.Stdout = pipeW
	ffmpeg.Stdin = pipeR

	outFile, err := os.Create(outputPath)
	if err != nil {
		m.log.Error(fmt.Sprintf("[%s] Cannot create output file: %s", stage, err))
		pipeR.Close()
		pipeW.Close()
		return
	}

	rec := &recording{
		stage:     stage,
		ytdlp:     ytdlp,
		ffmpeg:    ffmpeg,
		path:      outputPath,
		fileName:  fileName,
		listeners: listeners,
		lastCheck: time.Now(),
	}

	m.mu.Lock()
	if existing, ok := m.active[stage]; ok {
		m.mu.Unlock()
		pipeR.Close()
		pipeW.Close()
		_ = outFile.Close()
		_ = os.Remove(outputPath)
		existing.listeners = listeners
		return
	}
	m.active[stage] = rec
	m.mu.Unlock()

	m.log.Info(fmt.Sprintf("[%s] Starting recording...", stage))

	ytdlpStderr, err := ytdlp.StderrPipe()
	if err != nil {
		m.log.Error(fmt.Sprintf("[%s] yt-dlp pipe error: %s", stage, err))
		m.cleanupStartFailure(stage, rec, ytdlp, ffmpeg, pipeR, pipeW, outFile)
		return
	}
	ffmpegStdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		m.log.Error(fmt.Sprintf("[%s] ffmpeg stdout pipe error: %s", stage, err))
		ytdlpStderr.Close()
		m.cleanupStartFailure(stage, rec, ytdlp, ffmpeg, pipeR, pipeW, outFile)
		return
	}
	ffmpegStderr, err := ffmpeg.StderrPipe()
	if err != nil {
		m.log.Error(fmt.Sprintf("[%s] ffmpeg pipe error: %s", stage, err))
		ytdlpStderr.Close()
		ffmpegStdout.Close()
		m.cleanupStartFailure(stage, rec, ytdlp, ffmpeg, pipeR, pipeW, outFile)
		return
	}

	if err := ytdlp.Start(); err != nil {
		m.log.Error(fmt.Sprintf("[%s] Failed to start yt-dlp: %s", stage, err))
		ytdlpStderr.Close()
		ffmpegStdout.Close()
		ffmpegStderr.Close()
		m.cleanupStartFailure(stage, rec, ytdlp, ffmpeg, pipeR, pipeW, outFile)
		return
	}
	if err := ffmpeg.Start(); err != nil {
		m.log.Error(fmt.Sprintf("[%s] Failed to start ffmpeg: %s", stage, err))
		_ = ytdlp.Process.Kill()
		ytdlpStderr.Close()
		ffmpegStdout.Close()
		ffmpegStderr.Close()
		m.cleanupStartFailure(stage, rec, ytdlp, ffmpeg, pipeR, pipeW, outFile)
		return
	}

	m.wg.Add(1)
	go m.watchStderr(stage, "yt-dlp", ytdlpStderr)
	go m.watchStderr(stage, "ffmpeg", ffmpegStderr)
	go m.run(stage, rec, pipeR, pipeW, outFile, ffmpegStdout)
}

// cleanupStartFailure releases every resource allocated for a recording whose
// processes never started successfully.
func (m *Manager) cleanupStartFailure(stage string, rec *recording, ytdlp, ffmpeg *exec.Cmd, pipeR, pipeW, outFile *os.File) {
	pipeR.Close()
	pipeW.Close()
	_ = outFile.Close()
	_ = os.Remove(rec.path)
	m.remove(stage, rec)
}

// run orchestrates the lifetime of both processes and the output file.
// pipeW is yt-dlp's stdout; closing it once yt-dlp has exited hands ffmpeg the
// EOF it needs to finalize the MP3. ffmpeg's MP3 output is copied to the output
// file chunk by chunk so the file grows live.
func (m *Manager) run(stage string, rec *recording, pipeR, pipeW, outFile *os.File, ffStdout io.Reader) {
	defer m.wg.Done()

	// Copy ffmpeg's MP3 output straight to disk. io.Copy uses a small buffer
	// and each write is a direct syscall, so os.Stat reflects growth at once.
	copyDone := make(chan struct{})
	go func() {
		_, copyErr := io.Copy(outFile, ffStdout)
		_ = outFile.Close()
		if copyErr != nil && atomic.LoadInt32(&m.stopped) == 0 {
			m.log.Error(fmt.Sprintf("[%s] file write: %s", stage, copyErr))
		}
		close(copyDone)
	}()

	ytdlpErr := rec.ytdlp.Wait()
	pipeW.Close() // EOF -> ffmpeg finalizes the MP3

	ffmpegErr := rec.ffmpeg.Wait()
	<-copyDone
	pipeR.Close()

	if atomic.LoadInt32(&m.stopped) == 0 {
		if ytdlpErr != nil {
			if ee, ok := ytdlpErr.(*exec.ExitError); ok {
				m.log.Info(fmt.Sprintf("[%s] yt-dlp exited (code %d).", stage, ee.ExitCode()))
			} else {
				m.log.Error(fmt.Sprintf("[%s] yt-dlp: %s", stage, ytdlpErr))
			}
		}
		if ffmpegErr != nil {
			m.log.Error(fmt.Sprintf("[%s] ffmpeg: %s", stage, ffmpegErr))
		}
		if info, err := os.Stat(rec.path); err == nil && info.Size() == 0 {
			_ = os.Remove(rec.path)
			m.log.Warn(fmt.Sprintf("[%s] Removed empty recording.", stage))
		} else {
			m.log.Info(fmt.Sprintf("[%s] Recording finished.", stage))
		}
	}
	m.remove(stage, rec)
}

func (m *Manager) watchStderr(stage, source string, r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(strings.ToLower(line), "error") {
			m.log.Error(fmt.Sprintf("[%s] %s: %s", stage, source, line))
		}
	}
}

func (m *Manager) Stop(stage string) {
	m.mu.Lock()
	rec, ok := m.active[stage]
	delete(m.active, stage)
	m.mu.Unlock()
	if ok {
		rec.interrupt()
	}
}

func (m *Manager) forceKill(stage string) {
	m.mu.Lock()
	rec, ok := m.active[stage]
	delete(m.active, stage)
	m.mu.Unlock()
	if ok {
		rec.kill()
	}
}

// interrupt asks both processes to shut down gracefully so ffmpeg can finalize
// a valid MP3 (it flushes and writes the trailer on SIGINT).
func (r *recording) interrupt() {
	if runtime.GOOS == "windows" {
		// os.Interrupt is not delivered to child processes on Windows. Kill both
		// tools immediately so quitting the TUI cannot leave a hidden recorder
		// waiting for the force-kill timeout.
		r.kill()
		return
	}
	if r.ytdlp.Process != nil {
		_ = r.ytdlp.Process.Signal(os.Interrupt)
	}
	if r.ffmpeg.Process != nil {
		_ = r.ffmpeg.Process.Signal(os.Interrupt)
	}
}

// kill terminates both processes immediately (used for stalled recovery).
func (r *recording) kill() {
	if r.ytdlp.Process != nil {
		_ = r.ytdlp.Process.Kill()
	}
	if r.ffmpeg.Process != nil {
		_ = r.ffmpeg.Process.Kill()
	}
}

func (m *Manager) UpdateListeners(stage string, listeners int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.active[stage]; ok {
		rec.listeners = listeners
	}
}

func (m *Manager) MonitorStalled() {
	m.mu.RLock()
	stages := make([]string, 0, len(m.active))
	for stage := range m.active {
		stages = append(stages, stage)
	}
	m.mu.RUnlock()

	for _, stage := range stages {
		m.mu.RLock()
		rec := m.active[stage]
		m.mu.RUnlock()
		if rec == nil {
			continue
		}

		info, err := os.Stat(rec.path)
		if err != nil {
			if !os.IsNotExist(err) {
				m.log.Error(fmt.Sprintf("[%s] Stat error: %s", stage, err))
			}
			continue
		}

		rec.mu.Lock()
		stalled := false
		if info.Size() > rec.lastSize {
			rec.lastSize = info.Size()
			rec.lastCheck = time.Now()
		} else if time.Since(rec.lastCheck) > m.stalledAfter {
			stalled = true
		}
		rec.mu.Unlock()

		if stalled {
			m.log.Warn(fmt.Sprintf("[%s] Stalled. Restarting...", stage))
			m.forceKill(stage)
		}
	}
}

func (m *Manager) Active() []Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Snapshot, 0, len(m.active))
	for _, rec := range m.active {
		rec.mu.Lock()
		out = append(out, Snapshot{
			Stage:     rec.stage,
			Path:      rec.path,
			Listeners: rec.listeners,
		})
		rec.mu.Unlock()
	}
	return out
}

func (m *Manager) TotalListeners() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := 0
	for _, rec := range m.active {
		total += rec.listeners
	}
	return total
}

func (m *Manager) StopAll(timeout time.Duration) {
	atomic.StoreInt32(&m.stopped, 1)

	m.signalAll(os.Interrupt)

	m.log.Info("--- Gracefully shutting down ---")

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		m.signalAll(os.Kill)
		<-done
	}
}

func (m *Manager) signalAll(sig os.Signal) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rec := range m.active {
		if rec.ytdlp.Process != nil {
			_ = rec.ytdlp.Process.Signal(sig)
		}
		if rec.ffmpeg.Process != nil {
			_ = rec.ffmpeg.Process.Signal(sig)
		}
	}
}

// remove deletes the recording for stage only if it still refers to rec, so a
// freshly started recording is not clobbered by a late cleanup of a prior one.
func (m *Manager) remove(stage string, rec *recording) {
	m.mu.Lock()
	if cur := m.active[stage]; cur == rec {
		delete(m.active, stage)
	}
	m.mu.Unlock()
}
