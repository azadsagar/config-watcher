# Config Watcher

## Overview
`config-watcher` is a simple utility that monitors a configuration file for changes and restarts a specified daemon process when modifications are detected. It is designed for use in environments where the config file is on an NFS share, and inotify-based solutions are not viable.

## Features
- Periodically polls the config file for changes.
- Restarts the daemon process upon detecting a modification.
- Ensures the daemon shuts down gracefully before restarting.
- Exits immediately if the daemon crashes, instead of respawning it.
- Exits with the same exit code as the daemon process.
- Handles termination signals (`SIGINT`, `SIGTERM`) to cleanly shut down the daemon before exiting.

## Installation

```sh
# Clone the repository
git clone https://github.com/azadsagar/config-watcher.git
cd config-watcher

# Build the binary
go build -o config-watcher main.go
```

## Usage

```sh
./config-watcher -config=/mnt/nfs/config.yaml -cmd="/usr/local/bin/daemon" -interval=10s
```

### Command-line Arguments
| Argument     | Description                                              | Default Value |
|-------------|----------------------------------------------------------|---------------|
| `-config`   | Path to the configuration file to watch                  | `/path/to/config` |
| `-cmd`      | Path to the daemon command that needs to be restarted     | `/path/to/daemon` |
| `-interval` | Polling interval (e.g., `5s`, `1m`)                       | `5s`          |

## How It Works
1. Starts the daemon process.
2. Polls the config file at the specified interval.
3. If the config file is modified, the daemon process is gracefully restarted.
4. If the daemon crashes unexpectedly, the watcher exits immediately with the same exit code as the daemon.
5. If interrupted (`SIGINT` or `SIGTERM`), the watcher ensures the daemon shuts down cleanly before exiting.

## Example
Run the watcher with a 10-second polling interval:
```sh
./config-watcher -config=/mnt/nfs/config.yaml -cmd="/usr/local/bin/daemon" -interval=10s
```

## TO DO
- Add custom kill signal with command line flag to avoid sending default `SIGTERM` when config. Useful where child process has ability to reload itself instead of restarting.
- Watch remote configs (via apis) instead of watching local files.

## License
This project is licensed under the MIT License.

## Contributing
Feel free to open an issue or submit a pull request to enhance functionality or fix bugs.

