package filelog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/qjpcpu/filelog/diode"
)

// FileLogWriter log writer
type FileLogWriter interface {
	Write(p []byte) (int, error)
	Filename() string
	Truncate()
	Close() error
}

type fileLogWriter struct {
	*diode.Writer
	fwriter *fWriter
}

// Truncate file
func (fw *fileLogWriter) Truncate() {
	fw.fwriter.Truncate()
}

func (fw *fileLogWriter) Close() error {
	fw.Writer.Close()
	return fw.fwriter.Close()
}

// fWriter log writer
type fWriter struct {
	filename       string
	file           *os.File
	rt             RotateType
	realFilename   string
	createShortcut bool
	// flags
	reOpen        int32
	nonLinuxWatch int32
	truncateFlag  int32
	keepCount     int
}

type RotateType int

const (
	RotateDaily RotateType = iota
	RotateMinute
	RotateHourly
	RotateWeekly
	RotateNone
)

type Option struct {
	RotateType     RotateType
	CreateShortcut bool
	BufferSize     uint64
	FlushInterval  time.Duration
	KeepCount      int
}

type OptionWrapper func(*Option)

func RotateBy(t RotateType) OptionWrapper {
	return func(o *Option) {
		o.RotateType = t
	}
}

func CreateShortcut(yes bool) OptionWrapper {
	return func(o *Option) {
		o.CreateShortcut = yes
	}
}

func Keep(count int) OptionWrapper {
	return func(o *Option) {
		o.KeepCount = count
	}
}

// NewWriter create file logger, rotate none & by default
func NewWriter(filename string, wrappers ...OptionWrapper) (FileLogWriter, error) {
	f, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	opt := &Option{
		RotateType:     RotateNone,
		FlushInterval:  10 * time.Millisecond,
		BufferSize:     1024,
		CreateShortcut: false,
	}
	for _, fn := range wrappers {
		fn(opt)
	}
	if err = opt.validate(); err != nil {
		return nil, err
	}
	w := &fWriter{
		filename:       f,
		rt:             opt.RotateType,
		createShortcut: opt.CreateShortcut,
		reOpen:         1,
		keepCount:      opt.KeepCount,
	}
	wr := diode.NewWriter(w, int(opt.BufferSize), opt.FlushInterval, func(dropped int) {
		log.Printf("[filelog] %d logs dropped\n", dropped)
	})
	fw := &fileLogWriter{
		Writer:  &wr,
		fwriter: w,
	}
	return fw, nil
}

func (w *fWriter) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (opt *Option) validate() error {
	if !is2n(opt.BufferSize) {
		return fmt.Errorf("buffer size %d != 2^n", opt.BufferSize)
	}
	if opt.FlushInterval <= 0 {
		return fmt.Errorf("flush interval not set")
	}
	return nil
}

func logFilename(filename string, rt RotateType, now time.Time) string {
	switch rt {
	case RotateHourly:
		return fmt.Sprintf("%s.%s.%02d", filename, now.Format("2006-01-02"), now.Hour())
	case RotateMinute:
		return fmt.Sprintf("%s.%s.%02d.%02d", filename, now.Format("2006-01-02"), now.Hour(), now.Minute())
	case RotateWeekly:
		offset := int(now.Weekday()) - 1
		if offset < 0 {
			// sunday
			offset = 6
		}
		return fmt.Sprintf("%s.%s", filename, now.AddDate(0, 0, -offset).Format("2006-01-02"))
	case RotateNone:
		return filename
	default:
		// default rotate daily
		return fmt.Sprintf("%s.%s", filename, now.Format("2006-01-02"))
	}
}

func is2n(num uint64) bool {
	return num > 0 && num&(num-1) == 0
}

func (w *fWriter) needWatcher() bool {
	return w.rt == RotateNone
}

func (w *fileLogWriter) Filename() string { return w.fwriter.realFilename }

func (w *fWriter) openFile() error {
	// Open the log file
	w.realFilename = logFilename(w.filename, w.rt, time.Now())
	fd, err := os.OpenFile(w.realFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	w.file = fd
	if w.createShortcut && w.rt != RotateNone {
		linkto, _ := os.Readlink(w.filename)
		if linkto == "" || filepath.Base(linkto) != filepath.Base(w.realFilename) {
			os.Remove(w.filename)
			os.Symlink(filepath.Base(w.realFilename), w.filename)
		}
	}
	if w.needWatcher() {
		w.watchFile(w.realFilename)
	}
	return nil
}

func (w *fWriter) removeOldFile() {
	if w.keepCount <= 0 || w.rt == RotateNone {
		return
	}
	switch w.rt {
	case RotateDaily:
		os.Remove(logFilename(w.filename, w.rt, time.Now().AddDate(0, 0, -1*w.keepCount)))
	case RotateHourly:
		os.Remove(logFilename(w.filename, w.rt, time.Now().Add(-time.Hour*time.Duration(w.keepCount))))
	case RotateMinute:
		os.Remove(logFilename(w.filename, w.rt, time.Now().Add(-time.Minute*time.Duration(w.keepCount))))
	case RotateWeekly:
		os.Remove(logFilename(w.filename, w.rt, time.Now().AddDate(0, 0, -7*w.keepCount)))
	}
}

func (w *fWriter) doRotate() error {
	// Close any log file that may be open
	fd := w.file
	if fd != nil {
		fd.Close()
	}
	// Open the log file
	return w.openFile()
}

func (w *fWriter) needRotate() bool {
	return w.realFilename != logFilename(w.filename, w.rt, time.Now()) || w.reOpen == 1
}

func (w *fWriter) Truncate() {
	atomic.CompareAndSwapInt32(&w.truncateFlag, 0, 1)
}

func (w *fWriter) Write(p []byte) (int, error) {
	if w.needRotate() {
		if err := w.doRotate(); err != nil {
			fmt.Fprintf(os.Stderr, "fWriter(%q): %s\n", w.filename, err)
		}
		w.removeOldFile()
	}
	if w.truncateFlag == 1 && atomic.CompareAndSwapInt32(&w.truncateFlag, 1, 0) {
		w.file.Truncate(0)
		w.file.Seek(0, 0)
	}
	// Perform the write
	n, err := w.file.Write(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fWriter(%q): %s\n", w.filename, err)
	}
	return n, err
}
