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
	wg           sync.WaitGroup
)

var restartChan = make(chan struct{})

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

	wg.Add(1)

	go func(proc *os.Process) {
		defer wg.Done()

		exitChan := make(chan error, 1)

		// Monitor process exit
		go func() {
			exitChan <- cmd.Wait()
		}()

		select {
		case <-restartChan:
			log.Printf("Daemon with PID %d exited due to restart, continuing...", proc.Pid)
			return
		case err := <-exitChan:
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					log.Printf("Daemon with PID %d exited with status %d", proc.Pid, exitErr.ExitCode())
					os.Exit(exitErr.ExitCode()) // Exit if crashed
				} else {
					log.Printf("Daemon with PID %d exited with error: %v", proc.Pid, err)
					os.Exit(1)
				}
			} else {
				log.Printf("Daemon with PID %d exited cleanly.", proc.Pid)
				os.Exit(0)
			}
		}
	}(cmd.Process)

	return true
}

// restartDaemon stops the current daemon and starts a new one
func restartDaemon() bool {

	if cmd != nil && cmd.Process != nil {
		log.Println("Stopping daemon gracefully...")

		restartChan <- struct{}{}

		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			log.Printf("Failed to send SIGTERM: %v", err)
		}

		// Wait for the goroutine to confirm cleanup
		wg.Wait()
	}

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
