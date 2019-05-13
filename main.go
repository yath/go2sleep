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
	if len(args) < 1 {
		return errors.New("usage: send byte1 [byte2 [...]]")
	}

	data := make([]uint8, 0, len(args))
	for i, arg := range args {
		b, err := strconv.ParseUint(arg, 0, 8)
		if err != nil {
			return fmt.Errorf("can't convert args[%d]=%q to a uint8: %v", i, arg, err)
		}
		data = append(data, uint8(b))
	}

	return d.send(ctx, data)
}

func handleVersion(ctx context.Context, d *device, args []string) error {
	if len(args) > 0 {
		return errors.New("version does not take any arguments")
	}
	return d.send(ctx, []byte{0x5a, 0x14})
}

// receiveLoop connects to d, receives data from it until the context expires and prints the data.
func receiveLoop(ctx context.Context, d *device) error {
	if err := d.connect(ctx); err != nil {
		return fmt.Errorf("can't connect to device: %v", err)
	}

	rc, err := d.receive(ctx)
	if err != nil {
		return fmt.Errorf("can't set up receiver: %v", err)
	}

	for {
		select {
		case bytes := <-rc:
			if bytes == nil {
				return io.EOF
			}
			data, err := unmarshalData(bytes)
			if err != nil {
				log.Errorf("Can't unmarshal %d received bytes (%#v): %v.", len(bytes), bytes, err)
				continue
			}

			log.Noticef("Received data: %#v.", data)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	// Set up device and TUI.

	d := newDevice(*flagAddr)
	defer func() { warnOnError("Can't close device: %v.", d.close()) }()

	h := map[string]handlerFunc{
		"send":    handleSend,
		"version": handleVersion,
	}

	t, err := newTUI(h, d)
	if err != nil {
		log.Errorf("Can't initialize TUI: %v.", err)
		return
	}
	defer func() { warnOnError("Can't close TUI: %v.", t.close()) }()

	var wg sync.WaitGroup

	// TUI goroutine.
	wg.Add(1)
	go func() {
		if err := t.loop(ctx); err != nil && err != io.EOF {
			log.Errorf("Error processing terminal commands: %v.", err)
		}

		cancel()
		wg.Done()
	}()

	// Device goroutine.
	wg.Add(1)
	go func() {
		if err := receiveLoop(ctx, d); err != nil {
			log.Errorf("Error receiving data: %v.", err)
		}

		cancel()
		wg.Done()
	}()

	wg.Wait()
}
