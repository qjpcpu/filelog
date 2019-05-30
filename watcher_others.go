// +build darwin windows

// just for compile pass

package filelog

import (
	"os"
	"sync/atomic"
	"time"
)

func (w *fWriter) watchFile(filename string) {
	atomic.CompareAndSwapInt32(&w.reOpen, 1, 0)
	if atomic.CompareAndSwapInt32(&w.nonLinuxWatch, 0, 1) {
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
}
