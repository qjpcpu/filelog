// +build darwin windows

// just for compile pass

package filelog

import (
	"os"
	"sync/atomic"
	"time"
)

var shouldRunOnce int32

func runWatcherLoop(w *FileLogWriter, filename string) {
	ticker := time.NewTicker(3 * time.Second)
	go func() {
		for {
			<-ticker.C
			if _, err := os.Stat(filename); os.IsNotExist(err) {
				atomic.CompareAndSwapInt32(&w.reOpen, 0, 1)
			}
		}
	}()
}

func (w *FileLogWriter) watchFile(filename string) {
	atomic.CompareAndSwapInt32(&w.reOpen, 1, 0)
	if shouldRunOnce == 0 {
		runWatcherLoop(w, filename)
		atomic.CompareAndSwapInt32(&shouldRunOnce, 0, 1)
	}
}
