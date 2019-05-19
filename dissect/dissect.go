package main

/*
#cgo pkg-config: wireshark
#include <wireshark/config.h>
#include <wireshark/epan/packet.h>
*/
import "C"

func init() {
	protoPlugins = append(protoPlugins, &sleeponDissector{})
}

type sleeponDissector struct{}

var proto protocol

const (
	txCharacteristic = "6e400002-b5a3-f393-e0a9-e50e24dcca9e"
	rxCharacteristic = "6e400003-b5a3-f393-e0a9-e50e24dcca9e"
)

func (d *sleeponDissector) ProtoRegister() {
	proto = protoRegisterProtocol("SleepOn SLEEP2GO HST Protocol", "SleepOn", "sleepon")
}

func (d *sleeponDissector) ProtoRegHandoff() {
	hdl := createDissectorHandle(d, proto)
	for _, uuid := range []string{txCharacteristic, rxCharacteristic} {
		dissectorAddString("bluetooth.uuid", uuid, hdl)
	}
}

func (d *sleeponDissector) Dissect(tvb *C.tvbuff_t, pinfo *C.packet_info, tree *protoTree) int {
	tree.addProtocolF(proto, tvb, 0, -1, "%T%v.Dissect(%#v, %#v, %#v)", d, d, tvb, pinfo, tree)
	return 0
}
