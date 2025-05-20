/*
Copyright © 2024 Roberto Castro roberto.castro@example.com
*/
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/castrovroberto/codex-lite/cmd"
)

func main() {
	// Create a cancellable context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is called eventually.

	// Create a channel to receive OS signals.
	osSignalChan := make(chan os.Signal, 1)
	// Notify the channel for SIGINT (Ctrl+C) and SIGTERM (kill).
	signal.Notify(osSignalChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a channel to signal when cmd.ExecuteContext() is done.
	done := make(chan error, 1)

	go func() {
		// Pass the cancellable context to ExecuteContext.
		done <- cmd.ExecuteContext(ctx)
	}()

	// Wait for either the command to complete or a signal to be received.
	select {
	case err := <-done:
		// Command completed.
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error during command execution: %v\n", err)
			// os.Exit(1) will be handled by Cobra or the command itself if needed.
			// For now, just print the error. If ExecuteContext exits due to context
			// cancellation initiated by a signal, the error might be context.Canceled.
		} else {
			// fmt.Println("\nExiting normally.") // This might be too verbose if command has its own output.
		}
	case sig := <-osSignalChan:
		// Signal received, cancel the context.
		fmt.Printf("\nReceived signal: %s. Initiating shutdown...\n", sig)
		cancel() // This will propagate cancellation to commands using the context.

		// Wait for the command to acknowledge shutdown or timeout.
		// This gives cmd.ExecuteContext a chance to return.
		<-done // Or add a timeout here: time.After(...)
		fmt.Println("Shutdown complete.")
	}
}
