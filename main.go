/*
Copyright © 2025 Roberto Castro roberto.castro@example.com
*/
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/castrovroberto/CGE/cmd"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	osSignalChan := make(chan os.Signal, 1)
	signal.Notify(osSignalChan, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan error, 1)

	go func() {
		done <- cmd.ExecuteContext(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error during command execution: %v\n", err)
		}
	case sig := <-osSignalChan:
		fmt.Printf("\nReceived signal: %s. Initiating shutdown...\n", sig)
		cancel()

		<-done
		fmt.Println("Shutdown complete.")
	}
}
