#include <wireshark/config.h>
#include <wireshark/epan/packet.h>

#include <stdlib.h>

const char plugin_version[] = VERSION;
const char plugin_release[] = VERSION_RELEASE;

extern void protoRegisterAll(void);
extern void protoRegHandoffAll(void);

void plugin_register(void) {
    static proto_plugin plug;
    plug.register_protoinfo = protoRegisterAll;
    plug.register_handoff = protoRegHandoffAll;
    proto_register_plugin(&plug);
}

extern int callDissector(tvbuff_t *tvb, packet_info *pinfo, proto_tree *tree, void *data, void *k);

int call_callDissector(tvbuff_t *tvb, packet_info *pinfo, proto_tree *tree, void *data, void *k) {
    return callDissector(tvb, pinfo, tree, data, k);
}

proto_item *proto_tree_add_protocol_str(proto_tree *tree, int hfindex, tvbuff_t *tvb,
        gint start, gint length, const char *str) {
    return proto_tree_add_protocol_format(tree, hfindex, tvb, start, length, "%s", str);
}
