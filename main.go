package main

import (
	"context"
	"flag"
	"io"
	"sync"

	"github.com/op/go-logging"
)

var (
	flagAddr = flag.String("addr", "DE:7A:47:65:08:1F", "Bluetooth address of SleepOn device")
)

var log = logging.MustGetLogger("main")

type looper func(context.Context) error

// warnOnError prints a warning if the last element of args is non-nil.
func warnOnError(errfmt string, args ...interface{}) {
	if args[len(args)-1] == nil {
		return
	}
	log.Warningf(errfmt, args...)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	h := map[string]handlerFunc{}
	t, err := newTUI(h)
	if err != nil {
		log.Errorf("Can't initialize TUI: %v", err)
		return
	}
	defer func() { warnOnError("Can't close TUI: %v", t.close()) }()

	d := newDevice(*flagAddr)
	defer func() { warnOnError("Can't close device: %v", d.close()) }()

	var wg sync.WaitGroup
	for _, l := range []looper{t.loop, d.loop} {
		wg.Add(1)
		go func(ll looper) {
			if err := ll(ctx); err != nil && err != io.EOF {
				log.Errorf("Error during execution: %v", err)
			}
			cancel()
			wg.Done()
		}(l)
	}

	wg.Wait()
}
