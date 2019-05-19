package main

import (
	"fmt"
	"unsafe"
)

/*
#cgo pkg-config: wireshark
#include <wireshark/config.h>
#include <wireshark/epan/packet.h>

extern int call_callDissector(tvbuff_t *tvb, packet_info *pinfo, proto_tree *tree, void *data, void *k);
extern proto_item *proto_tree_add_protocol_str(proto_tree *tree, int hfindex, tvbuff_t *tvb, gint start, gint length, const char *str);
// This whole comment block must immediately precede the import "C" statement,
// without any empty lines, for it to be recognized.
*/
import "C"

// protoPlugin is a proto_plugin.
type protoPlugin interface {
	ProtoRegister()
	ProtoRegHandoff()
}

var protoPlugins []protoPlugin

type dissector interface {
	Dissect(tvb *C.tvbuff_t, pinfo *C.packet_info, tree *protoTree) int
}

var dissectors = map[unsafe.Pointer]dissector{}

func main() {
}

// Helpers.

func debug(f string, a ...interface{}) {
	fmt.Printf("DEBUG: "+f+"\n", a...)
}

func cstr(s string) *C.char {
	return C.CString(s)
}

func freestr(ss ...*C.char) {
	for _, s := range ss {
		C.free(unsafe.Pointer(s))
	}
}

func ptr(p unsafe.Pointer) *[0]byte {
	return (*[0]byte)(p)
}

// Called from C.

//export callDissector
func callDissector(tvb *C.tvbuff_t, pinfo *C.packet_info, tree *C.proto_tree, data, k unsafe.Pointer) C.int {
	d, ok := dissectors[k]
	if !ok {
		panic(fmt.Sprintf("callDissector called for unknown dissector %v, *%v = %q", k, k, C.GoString((*C.char)(k))))
	}
	return C.int(d.Dissect(tvb, pinfo, (*protoTree)(tree)))
}

//export protoRegisterAll
func protoRegisterAll() {
	for _, p := range protoPlugins {
		p.ProtoRegister()
	}
	debug("registered %d protoPlugins.", len(protoPlugins))
}

//export protoRegHandoffAll
func protoRegHandoffAll() {
	for _, p := range protoPlugins {
		p.ProtoRegHandoff()
	}
	debug("registered handoff for %d protoPlugins.", len(protoPlugins))
}

// Wrappers.

type protocol C.int

func (p protocol) c() C.int {
	return C.int(p)
}

func protoRegisterProtocol(name, shortName, filterName string) protocol {
	cn, cs, cf := cstr(name), cstr(shortName), cstr(filterName)
	defer freestr(cn, cs, cf)
	return protocol(C.proto_register_protocol(cn, cs, cf))
}

type dissectorHandle C.dissector_handle_t

func createDissectorHandle(d dissector, p protocol) dissectorHandle {
	k := unsafe.Pointer(cstr("dissector handle")) // Key. Just a random (but unique) pointer.
	name := cstr(fmt.Sprintf("Go dissector %v", k))

	dissectors[k] = d

	return dissectorHandle(C.register_dissector_with_data(name, ptr(C.call_callDissector), p.c(), k))
}

func dissectorAddString(name, pattern string, d dissectorHandle) {
	cn, cs := cstr(name), cstr(pattern)
	defer freestr(cn, cs)
	C.dissector_add_string(cn, cs, d)
}

type protoTree C.proto_tree

func (t *protoTree) c() *C.proto_tree {
	return (*C.proto_tree)(t)
}

func (t *protoTree) addProtocolF(hfindex protocol, tvb *C.tvbuff_t, start, length int, f string, a ...interface{}) *C.proto_item {
	str := cstr(fmt.Sprintf(f, a...))
	defer freestr(str)

	return C.proto_tree_add_protocol_str(t.c(), C.int(hfindex), tvb, C.int(start), C.int(length), str)
}
