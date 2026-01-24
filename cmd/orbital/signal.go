package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// setupSignalHandler creates a context that is cancelled when SIGINT or SIGTERM is received.
// It returns the context and a cleanup function that should be deferred.
func setupSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-sigChan:
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(sigChan)
	}()

	return ctx, cancel
}
