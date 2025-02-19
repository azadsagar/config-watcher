package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	configFile   string
	daemonCmd    string
	pollInterval time.Duration
	cmd          *exec.Cmd
	restarting   bool
	mu           sync.Mutex
)

func main() {
	// Parse command-line flags
	flag.StringVar(&configFile, "config", "/var/lib/grafana/plugins/watcher", "Path to the config file to watch")
	flag.StringVar(&daemonCmd, "cmd", "/run.sh", "Command to restart when config changes")
	flag.DurationVar(&pollInterval, "interval", 60*time.Second, "Polling interval (e.g., 5s, 2m)")
	flag.Parse()

	log.Printf("Watching %s every %s", configFile, pollInterval)

	// Get initial file modification time
	lastModTime := getFileModTime(configFile)

	// Start the daemon
	if !startDaemon() {
		log.Println("Failed to start daemon. Exiting watcher.")
		os.Exit(1)
	}

	// Handle termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received termination signal. Stopping daemon...")
		cleanupAndExit()
	}()

	// Poll for file changes
	for {
		time.Sleep(pollInterval)

		// Check if the config file modification time has changed
		newModTime := getFileModTime(configFile)
		if newModTime.After(lastModTime) {
			log.Println("Config change detected. Restarting daemon...")
			if !restartDaemon() {
				log.Println("Daemon failed to restart. Exiting watcher.")
				os.Exit(1)
			}
			lastModTime = newModTime
		}

		// Check if daemon has exited
		if !isDaemonRunning() {
			log.Println("Daemon crashed. Exiting watcher.")
			os.Exit(1)
		}
	}
}

// getFileModTime returns the last modification time of a file
func getFileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		log.Fatalf("Failed to stat file: %v", err)
	}
	return info.ModTime()
}

// startDaemon starts the daemon process
func startDaemon() bool {
	cmd = exec.Command(daemonCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start daemon: %v", err)
		return false
	}

	log.Printf("Started daemon with PID %d", cmd.Process.Pid)

	// Monitor daemon process exit immediately
	go func() {
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				log.Printf("Daemon exited with status %d", exitErr.ExitCode())
				os.Exit(exitErr.ExitCode()) // Exit with the same status as the daemon
			} else {
				log.Printf("Daemon exited with error: %v", err)
				os.Exit(1)
			}
		} else {
			log.Println("Daemon exited cleanly.")
			os.Exit(0) // Exit watcher if daemon exits cleanly
		}
	}()

	return true
}

// restartDaemon stops the current daemon and starts a new one
func restartDaemon() bool {
	mu.Lock()
	restarting = true
	mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		log.Println("Stopping daemon gracefully...")

		// Send SIGTERM for graceful shutdown
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			log.Printf("Failed to send SIGTERM: %v", err)
		}

		// Wait for process to exit, with a timeout
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-time.After(10 * time.Second): // Timeout
			log.Println("Daemon did not exit in time, force killing...")
			if err := cmd.Process.Kill(); err != nil {
				log.Printf("Failed to kill process: %v", err)
			}
			<-done // Ensure goroutine exits
		case err := <-done:
			if err != nil {
				log.Printf("inside restartDaemon method")
				log.Printf("Daemon exited with error: %v", err)
				//cleanupAndExit()
			} else {
				log.Println("Daemon exited cleanly.")
			}
		}
	}

	mu.Lock()
	restarting = false
	mu.Unlock()

	return startDaemon()
}

// isDaemonRunning checks if the daemon process is still running
func isDaemonRunning() bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}

	// Send signal 0 to check if process is alive
	err := cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// cleanupAndExit stops the daemon and exits
func cleanupAndExit() {
	if cmd != nil && cmd.Process != nil {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			log.Printf("Failed to send SIGTERM to process: %v", err)
		}
		cmd.Wait()
	}
	log.Println("Exiting watcher...")
	os.Exit(0)
}
