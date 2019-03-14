// +build linux
package filelog

import (
	"github.com/dersebi/golang_exp/exp/inotify"
	"path/filepath"
	"sync/atomic"
	"syscall"
)

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
