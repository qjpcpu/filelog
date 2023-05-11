# Features

* auto rotate by daily,hourly,minutely,none
* keep max KeepCount log files
* auto recreate log file when unexpected deletion

# Example

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
	w.Write([]byte("hello log"))
	w.Close()
}
```
