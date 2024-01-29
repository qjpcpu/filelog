//go:build linux
// +build linux

package filelog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"

	"github.com/dersebi/golang_exp/exp/inotify"
)

func (w *fWriter) watchFile() {
	wa, err := inotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "watch %v fail %v\n", w.filename, err)
		return
	}
	wa.AddWatch(filepath.Dir(w.filename), syscall.IN_DELETE)
	go func(iw *inotify.Watcher) {
		defer iw.Close()
		for {
			select {
			case ev := <-iw.Event:
				if ev.Mask == syscall.IN_DELETE {
					abs, _ := filepath.Abs(ev.Name)
					if abs == w.realFilename || abs == w.filename {
						atomic.StoreInt32(&w.reOpen, 1)
					}
				}
			case err := <-iw.Error:
				fmt.Fprintf(os.Stderr, "watch %v fail %v\n", w.filename, err)
				return
			case <-w.closeCh:
				return
			}
		}
	}(wa)
}
