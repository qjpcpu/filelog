
# example

```
package main

import (
	"github.com/qjpcpu/filelog"
	"log"
)

func main() {
	w, err := filelog.NewWriter("test.log", func(opt *filelog.Option){
        opt.RotateType = filelog.RotateDaily
        opt.CreateShortcut =  true
    })
	if err != nil {
		log.Fatal(err)
	}
	w.Write([]byte("hillo"))
	w.Close()
}
```
