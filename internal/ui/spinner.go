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
// The spinner appears on a new line, only if the action takes longer than 100ms.
func WithSpinner[T any](w io.Writer, message string, action func() (T, error)) (T, error) {
	done := make(chan struct{})
	var result T
	var err error

	// Run action in goroutine
	go func() {
		result, err = action()
		close(done)
	}()

	// Wait a bit before showing spinner (avoid flicker for fast operations)
	select {
	case <-done:
		return result, err
	case <-time.After(100 * time.Millisecond):
		// Action is taking a while, show spinner
	}

	frame := 0
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	// Print spinner on new line
	fmt.Fprintf(os.Stderr, "%s", spinnerFrames[frame])

	for {
		select {
		case <-done:
			// Clear spinner line
			fmt.Fprintf(os.Stderr, "\r\033[K")
			return result, err
		case <-ticker.C:
			frame = (frame + 1) % len(spinnerFrames)
			fmt.Fprintf(os.Stderr, "\r%s", spinnerFrames[frame])
		}
	}
}

// WithSpinnerErr is like WithSpinner but for actions that only return an error
func WithSpinnerErr(w io.Writer, message string, action func() error) error {
	_, err := WithSpinner(w, message, func() (struct{}, error) {
		return struct{}{}, action()
	})
	return err
}
