package main

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/godbus/dbus"

	btapi "github.com/muka/go-bluetooth/api"
	btprofile "github.com/muka/go-bluetooth/bluez/profile"
)

const retryDelay = 100 * time.Millisecond

const txCharacteristic = "6e400002-b5a3-f393-e0a9-e50e24dcca9e"
const rxCharacteristic = "6e400003-b5a3-f393-e0a9-e50e24dcca9e"

type device struct {
	addr string

	dev *btapi.Device
	txc *btprofile.GattCharacteristic1
	mu  sync.Mutex
}

// newDevice returns a new *device for the given address.
func newDevice(addr string) *device {
	return &device{addr: addr}
}

// connect connects to d.addr, retrying until the connection succeeds or the context expires.
// If d is already connected, this function does nothing.
func (d *device) connect(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.dev != nil {
		return nil
	}

	log.Infof("Connecting to %q", d.addr)
	dev, err := btapi.GetDeviceByAddress(d.addr)
	if err != nil {
		return fmt.Errorf("can't get device with address %q: %v", d.addr, err)
	}

	t := time.NewTicker(retryDelay)
	defer t.Stop()
F:
	for {
		err := dev.Connect()
		if err == nil {
			break F
		}
		log.Warningf("Can't connect to device %q: %v. Will retry.", d.addr, err)

		select {
		case <-t.C:
			continue F
		case <-ctx.Done():
			return fmt.Errorf("while connecting to %q: %v", d.addr, ctx.Err())
		}
	}

	d.dev = dev
	return nil
}

// getCharacteristic returns the characteristic determined by the given uuid, retrying until
// the characteristic is found or the context is cancelled.
func (d *device) getCharacteristic(ctx context.Context, uuid string) (*btprofile.GattCharacteristic1, error) {
	t := time.NewTicker(retryDelay)
	defer t.Stop()

F:
	for {
		c, err := d.dev.GetCharByUUID(uuid)
		if c != nil && err == nil {
			return c, nil
		}
		log.Warningf("Characteristic %q not yet found (error: %v). Will retry.", uuid, err)

		select {
		case <-t.C:
			continue F
		case <-ctx.Done():
			return nil, fmt.Errorf("while waiting for characteristic %q: %v", uuid, ctx.Err())
		}
	}
}

func handleRX(data []uint8) error {
	log.Debugf("Received %d bytes: %#v", len(data), data)
	if data[0] != 0x5b {
		return fmt.Errorf("first byte not 5b: %x", data[0])
	}

	switch data[1] {
	case 0x40: // heart_spo2
		pi := uint8(0)
		if len(data) > 7 {
			pi = data[7]
		}
		log.Noticef("HEART_SPO2{spo2: %d, heart: %d, wear: %d, charge: %d, pi: %d}", data[3], data[4], data[5], data[6], pi)
	case 0x15: // ble_battery
		log.Noticef("BATTERY{level: %d}", data[3])
	}
	return nil
}

// unpacks assigns the elements in []in to the values pointed to by []outps. in’s and
// outps’ lengths must match, and in[i] must be assignable to *out[i].
func unpack(in []interface{}, outps ...interface{}) error {
	if len(in) != len(outps) {
		return fmt.Errorf("got %d arguments, want %d", len(outps), len(in))
	}

	for i := range in {
		outpv := reflect.ValueOf(outps[i])
		if outpv.Kind() != reflect.Ptr {
			return fmt.Errorf("non-pointer output argument %d: %v", i, outpv.Type())
		}
		outv := outpv.Elem()

		inv := reflect.ValueOf(in[i])
		inT, outT := inv.Type(), outv.Type()
		if !inT.AssignableTo(outT) {
			return fmt.Errorf("argument %d is of type %v, want assignable to %v", i, inT, outT)
		}
		if !outv.CanSet() {
			return fmt.Errorf("argument %d (%#v) is not settable", i, outv)
		}
		outv.Set(inv)
	}

	return nil
}

// getChangedProperty returns a changed property, matching wantIface and wantProp, from
// a *dbus.Signal. If the signal does not match, (nil, false) is returned.
func getChangedProperty(s *dbus.Signal, wantIface, wantProp string) (interface{}, bool) {
	// https://dbus.freedesktop.org/doc/dbus-specification.html#standard-interfaces-properties
	if s.Name != "org.freedesktop.DBus.Properties.PropertiesChanged" {
		return nil, false
	}

	var iface string
	var changed map[string]dbus.Variant
	var invalidated []string
	if err := unpack(s.Body, &iface, &changed, &invalidated); err != nil {
		log.Errorf("can't get arguments from signal %v: %v", s, err)
		return nil, false
	}

	if iface != wantIface {
		return nil, false
	}

	v, ok := changed[wantProp]
	if !ok {
		return nil, false
	}

	return v.Value(), true
}

// loop reads data from the d’s UART RX characteristic until the context expires.
func (d *device) loop(ctx context.Context) error {
	if err := d.connect(ctx); err != nil {
		return fmt.Errorf("can't connect: %v", err)
	}

	char, err := d.getCharacteristic(ctx, rxCharacteristic)
	if err != nil {
		return fmt.Errorf("can't get RX characteristic: %v", err)
	}

	dc, err := char.Register()
	if err != nil {
		return fmt.Errorf("can't register for changes: %v", err)
	}

	go func() {
		for d := range dc {
			val, ok := getChangedProperty(d, "org.bluez.GattCharacteristic1", "Value")
			if !ok {
				log.Warningf("received unknown data: %#v", d)
				continue
			}

			if err := handleRX(val.([]uint8)); err != nil {
				log.Errorf("While handling %#v: %v", val, err)
			}
		}
	}()

	if err = char.StartNotify(); err != nil {
		return fmt.Errorf("can't start notifications: %v", err)
	}
	log.Debugf("Started notifications for characteristic %v.", char)

	defer func() {
		log.Debugf("Stopping notifications")
		warnOnError("can't stop notifications: %v", char.StopNotify())
	}()

	log.Infof("Listening for data")
	<-ctx.Done()

	return nil
}

// getTXCharacteristic retrieves the d’s UART TX characteristic if d.txc is non-nil and returns
// it.
func (d *device) getTXCharacteristic(ctx context.Context) (*btprofile.GattCharacteristic1, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.txc != nil {
		return d.txc, nil
	}

	txc, err := d.getCharacteristic(ctx, txCharacteristic)
	d.txc = txc
	return txc, err
}

// send writes data to d’s UART TX characteristic.
func (d *device) send(ctx context.Context, data []uint8) error {
	char, err := d.getTXCharacteristic(ctx)
	if err != nil {
		return fmt.Errorf("can't get TX characteristic: %v", err)
	}

	if err := char.WriteValue(data, nil); err != nil {
		return fmt.Errorf("can't write to TX characteristic %q: %v", txCharacteristic, err)
	}

	return nil
}

// close disconnects d.
func (d *device) close() error {
	log.Debugf("Disconnecting from %q", d.addr)
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d.dev.Disconnect()
	d.dev = nil
	return err
}
