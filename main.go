package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
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

func handleSend(ctx context.Context, d *device, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: send byte1 [byte2 [...]]")
	}

	data := make([]uint8, 0, len(args))
	for i := 1; i < len(args); i++ {
		b, err := strconv.ParseUint(args[i], 0, 8)
		if err != nil {
			return fmt.Errorf("can't convert args[%d]=%q to a uint8: %v", i, args[i], err)
		}
		data = append(data, uint8(b))
	}

	return d.send(ctx, data)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	d := newDevice(*flagAddr)
	defer func() { warnOnError("Can't close device: %v", d.close()) }()

	wrapDevice := func(hf func(context.Context, *device, []string) error) handlerFunc {
		return func(args []string) error {
			return hf(ctx, d, args)
		}
	}

	h := map[string]handlerFunc{
		"send": wrapDevice(handleSend),
	}

	t, err := newTUI(h)
	if err != nil {
		log.Errorf("Can't initialize TUI: %v", err)
		return
	}
	defer func() { warnOnError("Can't close TUI: %v", t.close()) }()

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
