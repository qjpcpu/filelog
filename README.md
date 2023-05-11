
# example

```
package main

import (
	"github.com/qjpcpu/filelog"
	"log"
)

func main() {
	w, err := filelog.NewWriter("test.log", 
        filelog.CreateShortcut(true),
        filelog.Keep(24),
        filelog.RotateBy(filelog.RotateHourly),
    )
	if err != nil {
		log.Fatal(err)
	}
	w.Write([]byte("hillo"))
	w.Close()
}
```
