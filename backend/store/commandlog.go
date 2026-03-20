package store

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const maxCommandBuffer = 1000

// Command is one entry from command-log.jsonl
type Command struct {
	TS   string `json:"ts"`
	Cmd  string `json:"cmd"`
	Exit int    `json:"exit"`
}

// CommandLog watches command-log.jsonl and maintains an in-memory ring buffer.
type CommandLog struct {
	mu       sync.RWMutex
	path     string
	commands []Command // ring buffer, newest at end
	offset   int64     // byte offset for incremental reads
}

// NewCommandLog creates a CommandLog for the given file path.
// The file may not exist yet — the watcher handles creation.
func NewCommandLog(path string) *CommandLog {
	return &CommandLog{path: path}
}

// Start begins watching the log file. Non-blocking (runs goroutine).
func (cl *CommandLog) Start() {
	// Do an initial read if the file already exists
	cl.readNew()

	go cl.watch()
}

func (cl *CommandLog) watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("commandlog: creating watcher: %v", err)
		return
	}
	defer watcher.Close()

	// Watch the parent directory so we catch file creation events
	dir := dirOf(cl.path)
	if err := watcher.Add(dir); err != nil {
		log.Printf("commandlog: watching dir %s: %v", dir, err)
		// Fall back to polling
		cl.pollLoop()
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Name == cl.path &&
				(event.Op&(fsnotify.Write|fsnotify.Create)) != 0 {
				cl.readNew()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("commandlog: watcher error: %v", err)
		}
	}
}

// pollLoop is a fallback for systems where fsnotify doesn't work.
func (cl *CommandLog) pollLoop() {
	for {
		cl.readNew()
		time.Sleep(2 * time.Second)
	}
}

// readNew reads any new lines appended since last read.
func (cl *CommandLog) readNew() {
	f, err := os.Open(cl.path)
	if err != nil {
		return // file doesn't exist yet — that's fine
	}
	defer f.Close()

	cl.mu.Lock()
	offset := cl.offset
	cl.mu.Unlock()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return
	}

	scanner := bufio.NewScanner(f)
	var newCmds []Command
	var newOffset int64 = offset

	for scanner.Scan() {
		line := scanner.Bytes()
		newOffset += int64(len(line)) + 1 // +1 for newline

		var cmd Command
		if err := json.Unmarshal(line, &cmd); err != nil {
			continue // skip malformed lines
		}
		newCmds = append(newCmds, cmd)
	}

	if len(newCmds) == 0 {
		return
	}

	cl.mu.Lock()
	cl.commands = append(cl.commands, newCmds...)
	// Trim to maxCommandBuffer
	if len(cl.commands) > maxCommandBuffer {
		cl.commands = cl.commands[len(cl.commands)-maxCommandBuffer:]
	}
	cl.offset = newOffset
	cl.mu.Unlock()
}

// GetRecent returns the most recent n commands (newest last).
func (cl *CommandLog) GetRecent(n int) []Command {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	if n <= 0 || n > len(cl.commands) {
		n = len(cl.commands)
	}
	result := make([]Command, n)
	copy(result, cl.commands[len(cl.commands)-n:])
	return result
}

// Len returns the number of buffered commands.
func (cl *CommandLog) Len() int {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	return len(cl.commands)
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
