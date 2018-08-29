package filelog

import (
	"fmt"
	"github.com/qjpcpu/filelog/diode"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"
)

type FileLogWriter struct {
	filename       string
	file           *os.File
	rt             RotateType
	realFilename   string
	createShortcut bool
}

type RotateType int

const (
	RotateDaily RotateType = iota
	RotateHourly
	RotateWeekly
)

func logFilename(filename string, rt RotateType) string {
	now := time.Now()
	switch rt {
	case RotateHourly:
		return fmt.Sprintf("%s.%s.%02d", filename, now.Format("2006-01-02"), now.Hour())
	case RotateWeekly:
		offset := int(now.Weekday()) - 1
		if offset < 0 {
			// sunday
			offset = 6
		}
		return fmt.Sprintf("%s.%s", filename, now.AddDate(0, 0, -offset).Format("2006-01-02"))
	default:
		// default rotate daily
		return fmt.Sprintf("%s.%s", filename, now.Format("2006-01-02"))
	}
}

func NewWriter(filename string, rt RotateType, createShortcut bool, params ...int) (io.WriteCloser, error) {
	f, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	filename = f
	w := &FileLogWriter{
		filename:       filename,
		rt:             rt,
		realFilename:   "",
		createShortcut: createShortcut,
	}
	bufSize := 1024
	if len(params) > 0 && params[0] > 0 {
		bufSize = params[0]
	}
	n := math.Log2(float64(bufSize))
	if n == math.Ceil(n) && n == math.Floor(n) {
		// valid
	} else {
		return nil, fmt.Errorf("buffer size %d != 2^n", bufSize)
	}
	wr := diode.NewWriter(w, bufSize, 10*time.Millisecond, func(dropped int) {
		log.Printf("[filelog] %d logs dropped\n", dropped)
	})
	return wr, nil
}

func (w *FileLogWriter) openFile() error {
	// Open the log file
	w.realFilename = logFilename(w.filename, w.rt)
	fd, err := os.OpenFile(w.realFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	w.file = fd
	if w.createShortcut {
		linkto, _ := os.Readlink(w.filename)
		if linkto == "" || filepath.Base(linkto) != filepath.Base(w.realFilename) {
			os.Remove(w.filename)
			os.Symlink(filepath.Base(w.realFilename), w.filename)
		}
	}
	return nil
}

func (w *FileLogWriter) doRotate() error {
	// Close any log file that may be open
	fd := w.file
	if fd != nil {
		fd.Close()
	}
	// Open the log file
	return w.openFile()
}

func (w *FileLogWriter) needRotate() bool {
	return w.realFilename != logFilename(w.filename, w.rt)
}

func (w *FileLogWriter) Write(p []byte) (int, error) {
	if w.needRotate() {
		if err := w.doRotate(); err != nil {
			fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.filename, err)
		}
	}
	// Perform the write
	n, err := w.file.Write(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.filename, err)
	}
	return n, err
}

func (w *FileLogWriter) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
