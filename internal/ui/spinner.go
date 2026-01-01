package ui

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Spinner frames for a simple dots animation
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// WithSpinner runs an action while displaying a spinner. Returns the result of the action.
// The spinner appears on a new line. If immediate is false, it waits 100ms before showing.
func WithSpinner[T any](w io.Writer, message string, immediate bool, action func() (T, error)) (T, error) {
	done := make(chan struct{})
	var result T
	var err error

	// Run action in goroutine
	go func() {
		result, err = action()
		close(done)
	}()

	// Wait a bit before showing spinner (avoid flicker for fast operations)
	if !immediate {
		select {
		case <-done:
			return result, err
		case <-time.After(100 * time.Millisecond):
			// Action is taking a while, show spinner
		}
	}

	frame := 0
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	// Print spinner
	if w == nil {
		w = os.Stderr
	}
	
	fmt.Fprintf(w, "\r%s %s", message, spinnerFrames[frame])

	for {
		select {
		case <-done:
			// Clear spinner line
			fmt.Fprintf(w, "\r\033[K")
			return result, err
		case <-ticker.C:
			frame = (frame + 1) % len(spinnerFrames)
			fmt.Fprintf(w, "\r%s %s", message, spinnerFrames[frame])
		}
	}
}

// WithSpinnerErr is like WithSpinner but for actions that only return an error
func WithSpinnerErr(w io.Writer, message string, immediate bool, action func() error) error {
	_, err := WithSpinner(w, message, immediate, func() (struct{}, error) {
		return struct{}{}, action()
	})
	return err
}
