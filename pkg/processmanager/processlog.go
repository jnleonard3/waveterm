// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package processmanager

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	logLimit        = 400
	maxLogFileBytes = 5 * 1024 * 1024
	logDirPerm      = 0700
	logFilePerm     = 0600
)

const (
	streamStdout = iota
	streamStderr
	streamCount
)

// processLog keeps the last logLimit lines in memory, numbered by a
// monotonically increasing sequence, and mirrors every complete line to a
// size-capped log file. It is not safe for concurrent use; the owning
// supervisor serializes access.
type processLog struct {
	path    string
	lines   []string
	seq     int64
	partial [streamCount]string
	file    *os.File
	size    int64
}

func (l *processLog) write(stream int, data string) {
	buf := l.partial[stream] + data
	for {
		idx := strings.IndexByte(buf, '\n')
		if idx < 0 {
			break
		}
		l.appendLine(strings.TrimSuffix(buf[:idx], "\r"))
		buf = buf[idx+1:]
	}
	l.partial[stream] = buf
}

func (l *processLog) flush() {
	for stream := range l.partial {
		if l.partial[stream] == "" {
			continue
		}
		l.appendLine(l.partial[stream])
		l.partial[stream] = ""
	}
}

func (l *processLog) appendLine(line string) {
	l.seq++
	l.lines = append(l.lines, line)
	if len(l.lines) > logLimit {
		l.lines = append([]string(nil), l.lines[len(l.lines)-logLimit:]...)
	}
	l.writeFile(line + "\n")
}

// since returns the lines newer than sinceSeq. A sinceSeq ahead of the current
// sequence means the log was reset, so the caller gets every line.
func (l *processLog) since(sinceSeq int64) []string {
	if sinceSeq < 0 || sinceSeq > l.seq {
		sinceSeq = 0
	}
	fresh := int(min(l.seq-sinceSeq, int64(len(l.lines))))
	return append([]string{}, l.lines[len(l.lines)-fresh:]...)
}

func (l *processLog) writeFile(data string) {
	if l.path == "" {
		return
	}
	if l.file == nil && !l.openFile() {
		return
	}
	if l.size+int64(len(data)) > maxLogFileBytes {
		l.rotateFile()
		if l.file == nil {
			return
		}
	}
	n, err := l.file.WriteString(data)
	l.size += int64(n)
	if err != nil {
		log.Printf("processmanager: error writing %s: %v\n", l.path, err)
		l.closeFile()
	}
}

func (l *processLog) openFile() bool {
	if err := os.MkdirAll(filepath.Dir(l.path), logDirPerm); err != nil {
		log.Printf("processmanager: error creating log dir: %v\n", err)
		return false
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, logFilePerm)
	if err != nil {
		log.Printf("processmanager: error opening %s: %v\n", l.path, err)
		return false
	}
	info, err := f.Stat()
	if err == nil {
		l.size = info.Size()
	}
	l.file = f
	return true
}

func (l *processLog) rotateFile() {
	l.closeFile()
	if err := os.Rename(l.path, l.path+".1"); err != nil {
		log.Printf("processmanager: error rotating %s: %v\n", l.path, err)
	}
	l.openFile()
}

func (l *processLog) closeFile() {
	if l.file != nil {
		_ = l.file.Close()
	}
	l.file, l.size = nil, 0
}
