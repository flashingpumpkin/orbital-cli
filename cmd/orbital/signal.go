package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// setupSignalHandler creates a context that is cancelled when SIGINT or SIGTERM is received.
// It returns the context and a cleanup function that should be deferred.
// On the first signal, the context is cancelled for graceful shutdown.
// On the second signal, the process exits immediately.
func setupSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		// First signal: cancel context for graceful shutdown
		select {
		case <-sigChan:
			cancel()
		case <-ctx.Done():
			signal.Stop(sigChan)
			return
		}

		// Second signal: force exit
		select {
		case <-sigChan:
			fmt.Fprintln(os.Stderr, "\nForce exit")
			os.Exit(130)
		case <-ctx.Done():
		}
		signal.Stop(sigChan)
	}()

	return ctx, cancel
}
