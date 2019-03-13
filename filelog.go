package filelog

import (
	"fmt"
	"github.com/dersebi/golang_exp/exp/inotify"
	"github.com/qjpcpu/filelog/diode"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
)

// FileLogWriter log writer
type FileLogWriter struct {
	filename       string
	file           *os.File
	rt             RotateType
	realFilename   string
	createShortcut bool
	reOpen         int32
}

// RotateType 轮转类型
type RotateType int

const (
	// RotateDaily 按天轮转
	RotateDaily RotateType = iota
	// RotateHourly 按小时轮转
	RotateHourly
	// RotateWeekly 按周轮转
	RotateWeekly
	// RotateNone 不切割日志
	RotateNone
)

// Option 参数选项
type Option struct {
	RotateType     RotateType
	CreateShortcut bool
	BufferSize     uint64
	FlushInterval  time.Duration
}

// OptionWrapper 参数配置函数
type OptionWrapper func(*Option)

// NewWriter 创建文件日志,默认选项日志不会自动轮转
func NewWriter(filename string, wrappers ...OptionWrapper) (io.WriteCloser, error) {
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
	w := &FileLogWriter{
		filename:       f,
		rt:             opt.RotateType,
		createShortcut: opt.CreateShortcut,
	}
	wr := diode.NewWriter(w, int(opt.BufferSize), opt.FlushInterval, func(dropped int) {
		log.Printf("[filelog] %d logs dropped\n", dropped)
	})
	return wr, nil
}

// Close 关闭文件
func (w *FileLogWriter) Close() error {
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
	case RotateNone:
		return filename
	default:
		// default rotate daily
		return fmt.Sprintf("%s.%s", filename, now.Format("2006-01-02"))
	}
}

func is2n(num uint64) bool {
	if num == 0 {
		return false
	}
	var bit uint64 = 1
	var i uint64 = 0
	for ; i < 64; i++ {
		if m := bit << i; (^m)&num == 0 {
			return true
		} else if m > num {
			return false
		}
	}
	return false
}

func (w *FileLogWriter) needWatcher() bool {
	return w.rt == RotateNone
}

func (w *FileLogWriter) watchFile(filename string) {
	for doOnce := true; doOnce; doOnce = false {
		atomic.CompareAndSwapInt32(&w.reOpen, 1, 0)
		wa, err := inotify.NewWatcher()
		if err != nil {
			break
		}
		if err = wa.AddWatch(filename, syscall.IN_DELETE|syscall.IN_DELETE_SELF|syscall.IN_MOVE|syscall.IN_MOVE_SELF|syscall.IN_IGNORED); err != nil {
			break
		}
		wa.AddWatch(filepath.Dir(filename), syscall.IN_DELETE)
		go func(iw *inotify.Watcher) {
		LOOP:
			for {
				select {
				case ev := <-iw.Event:
					if ev.Mask == syscall.IN_DELETE {
						abs, _ := filepath.Abs(ev.Name)
						if abs == filename {
							break LOOP
						}
					} else {
						break LOOP
					}
				case <-iw.Error:
					break LOOP
				}
			}
			atomic.CompareAndSwapInt32(&w.reOpen, 0, 1)
			iw.Close()
		}(wa)
	}
}

func (w *FileLogWriter) openFile() error {
	// Open the log file
	w.realFilename = logFilename(w.filename, w.rt)
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
	return w.realFilename != logFilename(w.filename, w.rt) || w.reOpen == 1
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
