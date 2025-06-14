package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// logWriter writes log entries to a file with simple size-based rotation.
type logWriter struct {
	mu   sync.Mutex
	file *os.File
	size int64
	dir  string
	base string
	ch   chan string
	wg   sync.WaitGroup
}

func newLogWriter(dir, base string) (*logWriter, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lw := &logWriter{dir: dir, base: base, ch: make(chan string, 100)}
	if err := lw.open(); err != nil {
		return nil, err
	}
	lw.wg.Add(1)
	go lw.loop()
	return lw, nil
}

func (lw *logWriter) loop() {
	defer lw.wg.Done()
	for entry := range lw.ch {
		lw.write(entry)
	}
}

func (lw *logWriter) write(entry string) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	if lw.file == nil {
		if err := lw.open(); err != nil {
			return
		}
	}
	n, _ := lw.file.WriteString(entry + "\n")
	lw.size += int64(n)
	if lw.size > 1<<20 { // 1MB
		_ = lw.rotate()
	}
}

func (lw *logWriter) open() error {
	path := filepath.Join(lw.dir, lw.base+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	lw.file = f
	info, err := f.Stat()
	if err == nil {
		lw.size = info.Size()
	}
	return nil
}

func (lw *logWriter) rotate() error {
	lw.file.Close()
	ts := time.Now().Format("20060102-150405")
	old := filepath.Join(lw.dir, fmt.Sprintf("%s-%s.log", lw.base, ts))
	os.Rename(filepath.Join(lw.dir, lw.base+".log"), old)
	lw.size = 0
	return lw.open()
}

func (lw *logWriter) Write(entry string) {
	select {
	case lw.ch <- entry:
	default:
		lw.write(entry)
	}
}

// Close flushes pending log entries and closes the file.
func (lw *logWriter) Close() {
	close(lw.ch)
	lw.wg.Wait()
	lw.mu.Lock()
	defer lw.mu.Unlock()
	if lw.file != nil {
		lw.file.Close()
		lw.file = nil
	}
}
