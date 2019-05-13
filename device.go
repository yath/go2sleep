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

// retryDelay is the delay before a failed connect() or getCharacteristic() is retried.
const retryDelay = 100 * time.Millisecond

// Nordic UART Service TX and RX (from client’s PoV) UUIDs.
const (
	txCharacteristic = "6e400002-b5a3-f393-e0a9-e50e24dcca9e"
	rxCharacteristic = "6e400003-b5a3-f393-e0a9-e50e24dcca9e"
)

// device is a BLE device providing the Nordic UART service.
type device struct {
	addr string

	dev *btapi.Device
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

	log.Infof("Connecting to %q.", d.addr)
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

// unpack assigns the elements in []in to the values pointed to by []outps. in’s and
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
		log.Errorf("can't get arguments from signal %v: %v.", s, err)
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

// receive subscribes to d’s UART RX characteristic and returns a channel emitting the received data.
func (d *device) receive(ctx context.Context) (<-chan []byte, error) {
	rx, err := d.getCharacteristic(ctx, rxCharacteristic)
	if err != nil {
		return nil, fmt.Errorf("can't get RX characteristic: %v", err)
	}

	sigc, err := rx.Register()
	if err != nil {
		return nil, fmt.Errorf("can't register for changes: %v", err)
	}

	if err = rx.StartNotify(); err != nil {
		return nil, fmt.Errorf("can't start notifications: %v", err)
	}
	log.Debugf("Started notifications for characteristic %#v.", rx)

	ret := make(chan []byte, 1)
	go func() {
		defer func() {
			close(ret)
			log.Debugf("Stopping notifications for characteristic %#v.", rx)
			warnOnError("Can't stop notifications: %v.", rx.StopNotify())
		}()

		for {
			select {
			case sig := <-sigc:
				if sig == nil {
					return
				}
				val, ok := getChangedProperty(sig, "org.bluez.GattCharacteristic1", "Value")
				if !ok {
					log.Warningf("Received unknown data: %#v.", sig)
					continue
				}

				valb, ok := val.([]byte)
				if !ok {
					log.Warningf("GattCharacteristic1’s value is not a []byte but %T: %#v.", val, val)
					continue
				}

				ret <- valb
			case <-ctx.Done():
				return
			}
		}
	}()

	return ret, nil
}

// send writes data to d’s UART TX characteristic.
func (d *device) send(ctx context.Context, data []byte) error {
	tx, err := d.getCharacteristic(ctx, txCharacteristic)
	if err != nil {
		return fmt.Errorf("can't get TX characteristic: %v", err)
	}

	if err := tx.WriteValue(data, nil); err != nil {
		return fmt.Errorf("can't write to TX characteristic %q: %v", txCharacteristic, err)
	}

	return nil
}

// close disconnects d.
func (d *device) close() error {
	log.Debugf("Disconnecting from %q.", d.addr)
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d.dev.Disconnect()
	d.dev = nil
	return err
}
