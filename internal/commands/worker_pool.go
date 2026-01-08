package commands

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gYonder/drime-shell/internal/api"
)

// UploadConfig holds configuration for directory uploads
type UploadConfig struct {
	Concurrency   int           // Number of parallel uploads (default: 6)
	RetryAttempts int           // Number of retry attempts per file (default: 10)
	RetryDelay    time.Duration // Base delay between retries (default: 2s)
	APIDelay      time.Duration // Delay between API calls to avoid rate limiting (default: 100ms)
	Timeout       time.Duration // Timeout per upload attempt (default: 40s)
}

// DefaultUploadConfig returns sensible defaults
func DefaultUploadConfig() UploadConfig {
	return UploadConfig{
		Concurrency:   6,
		RetryAttempts: 10,
		RetryDelay:    2 * time.Second,
		APIDelay:      100 * time.Millisecond,
		Timeout:       40 * time.Second,
	}
}

// UploadStats tracks upload statistics
type UploadStats struct {
	Errors   []UploadError
	Uploaded int64
	Skipped  int64
	Failed   int64
	mu       sync.Mutex
}

// UploadError represents a failed upload
type UploadError struct {
	Path  string
	Error string
}

func (s *UploadStats) AddUploaded() {
	atomic.AddInt64(&s.Uploaded, 1)
}

func (s *UploadStats) AddSkipped() {
	atomic.AddInt64(&s.Skipped, 1)
}

func (s *UploadStats) AddFailed(path, errMsg string) {
	atomic.AddInt64(&s.Failed, 1)
	s.mu.Lock()
	s.Errors = append(s.Errors, UploadError{Path: path, Error: errMsg})
	s.mu.Unlock()
}

// FileUploadTask represents a file to upload
type FileUploadTask struct {
	LocalPath    string // Full local path
	RelativePath string // Path relative to upload root
	ParentID     int64  // Remote parent folder ID
	Size         int64  // File size
}

// UploadProgress tracks overall progress
type UploadProgress struct {
	StartTime time.Time
	Total     int64
	Completed int64
}

func (p *UploadProgress) Increment() int64 {
	return atomic.AddInt64(&p.Completed, 1)
}

func (p *UploadProgress) Percent() int {
	if p.Total == 0 {
		return 100
	}
	return int(float64(p.Completed) / float64(p.Total) * 100)
}

func (p *UploadProgress) ETA() string {
	completed := atomic.LoadInt64(&p.Completed)
	if completed == 0 {
		return "calculating..."
	}
	elapsed := time.Since(p.StartTime)
	itemsPerSecond := float64(completed) / elapsed.Seconds()
	remaining := p.Total - completed
	if itemsPerSecond <= 0 {
		return "calculating..."
	}
	eta := time.Duration(float64(remaining)/itemsPerSecond) * time.Second
	if eta < time.Minute {
		return fmt.Sprintf("%ds", int(eta.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(eta.Minutes()), int(eta.Seconds())%60)
}

// WorkerPool manages concurrent file uploads
type WorkerPool struct {
	ctx         context.Context
	client      api.DrimeClient
	tasks       chan FileUploadTask
	stats       *UploadStats
	progress    *UploadProgress
	cache       *api.FileCache
	session     *UploadSession
	onProgress  func(completed, total int64, percent int, eta string)
	onFile      func(relativePath string, success bool, err string)
	basePath    string // Remote base path for cache updates
	workspaceID int64  // Workspace ID for uploads
	wg          sync.WaitGroup
	config      UploadConfig
}

// NewWorkerPool creates a new upload worker pool
func NewWorkerPool(
	ctx context.Context,
	client api.DrimeClient,
	cache *api.FileCache,
	basePath string,
	config UploadConfig,
	session *UploadSession,
	workspaceID int64,
) *WorkerPool {
	if config.Concurrency <= 0 {
		config.Concurrency = 6
	}
	if config.RetryAttempts <= 0 {
		config.RetryAttempts = 10
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 2 * time.Second
	}
	if config.Timeout <= 0 {
		config.Timeout = 40 * time.Second
	}

	return &WorkerPool{
		ctx:         ctx,
		client:      client,
		config:      config,
		tasks:       make(chan FileUploadTask, config.Concurrency*2), // Buffered channel
		stats:       &UploadStats{},
		progress:    &UploadProgress{StartTime: time.Now()},
		cache:       cache,
		basePath:    basePath,
		session:     session,
		workspaceID: workspaceID,
	}
}

// SetCallbacks sets progress and file completion callbacks
func (wp *WorkerPool) SetCallbacks(
	onProgress func(completed, total int64, percent int, eta string),
	onFile func(relativePath string, success bool, err string),
) {
	wp.onProgress = onProgress
	wp.onFile = onFile
}

// Start launches worker goroutines
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.config.Concurrency; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Submit adds a task to the upload queue
func (wp *WorkerPool) Submit(task FileUploadTask) {
	atomic.AddInt64(&wp.progress.Total, 1)
	wp.tasks <- task
}

// Close signals no more tasks and waits for completion
func (wp *WorkerPool) Close() *UploadStats {
	close(wp.tasks)
	wp.wg.Wait()
	return wp.stats
}

// worker processes upload tasks
func (wp *WorkerPool) worker(_ int) {
	defer wp.wg.Done()

	for task := range wp.tasks {
		select {
		case <-wp.ctx.Done():
			return
		default:
		}

		err := wp.uploadWithRetry(task)

		completed := wp.progress.Increment()
		if wp.onProgress != nil {
			wp.onProgress(completed, wp.progress.Total, wp.progress.Percent(), wp.progress.ETA())
		}

		if err != nil {
			wp.stats.AddFailed(task.RelativePath, err.Error())
			if wp.onFile != nil {
				wp.onFile(task.RelativePath, false, err.Error())
			}
			// Update session state
			if wp.session != nil {
				wp.session.MarkFileFailed(task.RelativePath, err.Error())
				_ = wp.session.Save() // Best effort save
			}
		} else {
			wp.stats.AddUploaded()
			if wp.onFile != nil {
				wp.onFile(task.RelativePath, true, "")
			}
			// Update session state
			if wp.session != nil {
				wp.session.MarkFileCompleted(task.RelativePath, task.Size)
				_ = wp.session.Save() // Best effort save
			}
		}

		// API delay to avoid rate limiting
		if wp.config.APIDelay > 0 {
			time.Sleep(wp.config.APIDelay)
		}
	}
}

// uploadWithRetry attempts to upload a file with retries
func (wp *WorkerPool) uploadWithRetry(task FileUploadTask) error {
	var lastErr error

	for attempt := 1; attempt <= wp.config.RetryAttempts; attempt++ {
		// Create timeout context for this attempt
		attemptCtx, cancel := context.WithTimeout(wp.ctx, wp.config.Timeout)
		err := wp.uploadFile(attemptCtx, task)
		cancel()

		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on parent context cancellation
		if wp.ctx.Err() != nil {
			return wp.ctx.Err()
		}

		// Don't retry on the last attempt
		if attempt < wp.config.RetryAttempts {
			// Exponential backoff with jitter
			backoff := float64(wp.config.RetryDelay) * math.Pow(2, float64(attempt-1))
			jitter := rand.Float64() * 0.25 * backoff
			sleepDuration := time.Duration(backoff + jitter)

			// Cap at 30 seconds
			if sleepDuration > 30*time.Second {
				sleepDuration = 30 * time.Second
			}

			select {
			case <-time.After(sleepDuration):
			case <-wp.ctx.Done():
				return wp.ctx.Err()
			}
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", wp.config.RetryAttempts, lastErr)
}

// uploadFile performs the actual upload
func (wp *WorkerPool) uploadFile(ctx context.Context, task FileUploadTask) error {
	f, err := os.Open(task.LocalPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	parentID := &task.ParentID

	entry, err := wp.client.Upload(ctx, f, filepath.Base(task.LocalPath), parentID, task.Size, wp.workspaceID)
	if err != nil {
		return err
	}

	// Update cache
	if entry != nil && wp.cache != nil {
		remotePath := filepath.Join(wp.basePath, task.RelativePath)
		wp.cache.Add(entry, remotePath)
	}

	return nil
}

// ProgressPrinter provides simple console progress output
type ProgressPrinter struct {
	lastLine string
	mu       sync.Mutex
}

func NewProgressPrinter() *ProgressPrinter {
	return &ProgressPrinter{}
}

func (pp *ProgressPrinter) OnProgress(completed, total int64, percent int, eta string) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	// Clear previous line and print progress
	line := fmt.Sprintf("\r  Progress: %d/%d (%d%%) - ETA: %s", completed, total, percent, eta)
	// Pad with spaces to clear any previous longer text
	if len(line) < len(pp.lastLine) {
		line += strings.Repeat(" ", len(pp.lastLine)-len(line))
	}
	fmt.Print(line)
	pp.lastLine = line
}

func (pp *ProgressPrinter) OnFile(relativePath string, success bool, errMsg string) {
	pp.mu.Lock()
	defer pp.mu.Unlock()

	// Print file result on new line
	if !success {
		fmt.Printf("\r  ✗ %s: %s\n", relativePath, errMsg)
		pp.lastLine = "" // Reset so progress doesn't try to clear file output
	}
}

func (pp *ProgressPrinter) Finish() {
	fmt.Println() // New line after progress
}
