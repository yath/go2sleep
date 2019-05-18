TX_UUID = "6e400002-b5a3-f393-e0a9-e50e24dcca9e"
RX_UUID = "6e400003-b5a3-f393-e0a9-e50e24dcca9e"

sleepon = Proto("sleepon", "SleepOn SLEEP2GO HST Protocol")

local pf_direction_values = {
    [0x5a]="Host to Device",
    [0x5b]="Device to Host",
}
local pf_direction = ProtoField.uint8("sleepon.direction", "Direction", base.HEX, pf_direction_values)

local pf_cmd_values = {
    [0x02]="BLE_TYPE",
    [0x03]="BLE_SN",
    [0x0b]="HISTORY_SLEEP_INDEX",
    [0x0c]="HISTORY_STEPS_DATA",
    [0x0f]="CURRENT_HEART",
    [0x12]="REAL_HEART_SWITCH",
    [0x14]="BLE_VERSION",
    [0x15]="BLE_BATTERY",
    [0x1b]="CLEAR_BLE_FLASH",
    [0x1d]="GET_BLE_TIME",
    [0x1e]="SET_BLE_TIME",
    [0x1f]="BLE_RESET",
    [0x20]="OPEN_LIGHT_COLOR",
    [0x21]="CLOSE_LIGHT",
    [0x22]="OPEN_SHAKE",
    [0x23]="CLOSE_SHAKE",
    [0x25]="SHAKE_MODE",
    [0x29]="SENSOR_RATE",
    [0x30]="HEART_SPO2_SWITCH",
    [0x32]="WRITE_SN",
    [0x33]="BLE_RESTART",
    [0x34]="BLE_CLOCK",
    [0x35]="CLEAR_FLASH_DATA",
    [0x38]="HISTORY_SUMMARY",
    [0x3b]="MINUTE_DATA",
    [0x3c]="SECOND_DATA",
    [0x3e]="MINUTE_DATA_INDEX",
    [0x3f]="SECOND_DATA_INDEX",
    [0x40]="HEART_SPO2",
    [0x41]="PULSE",
    [0x42]="HOUR_AHI",
    [0x43]="LOW_OXYGEN",
    [0x44]="SHAKE",
    [0xe0]="FM_RESET",
    [0xe1]="FM_ERROR",
}
local pf_cmd = ProtoField.uint8("sleepon.cmd", "Command", base.HEX, pf_cmd_values)

local pf_status = ProtoField.uint8("sleepon.status", "Status", base.HEX) -- maybe

local pf_battery_level = ProtoField.uint8("sleepon.battery.level", "Battery Percentage", base.DEC)

local pf_heartspo2 = ProtoField.bytes("sleepon.heart_spo2", "Heart/SpO2 Data")
local pf_heartspo2_spo2 = ProtoField.uint8("sleepon.heart_spo2.spo2", "SpO2 Percentage", base.DEC)
local pf_heartspo2_hr = ProtoField.uint8("sleepon.heart_spo2.heart_rate", "Heart Rate", base.DEC)
local pf_heartspo2_wearing = ProtoField.bool("sleepon.heart_spo2.wearing", "Wearing")
local pf_heartspo2_charging_values = {[0]="No", "Yes", "Full"}
local pf_heartspo2_charging = ProtoField.uint8("sleepon.heart_spo2.charging", "Charging", base.HEX, pf_heartspo2_charging_values)
local pf_heartspo2_pi = ProtoField.uint8("sleepon.heart_spo2.pi", "PI", base.DEC)

local pf_minute_data = ProtoField.bytes("sleepon.minute_data", "Minute Data")
local pf_minute_data_num = ProtoField.uint16("sleepon.minute_data.num", "Number")
local pf_minute_data_index = ProtoField.uint16("sleepon.minute_data.index", "Index")
local pf_minute_data_data_count = ProtoField.uint16("sleepon.minute_data.h_data_count", "h_data_count")
local pf_minute_data_data_package = ProtoField.uint16("sleepon.minute_data.h_data_package", "h_data_package")
local pf_minute_data_synchronize = ProtoField.uint16("sleepon.minute_data.synchronize", "synchronize")
local pf_minute_data_period = ProtoField.bytes("sleepon.minute_data.period", "period FIXME")
local pf_minute_data_minute_heart = ProtoField.uint8("sleepon.minute_data.minute_heart", "Minute Heart")
local pf_minute_data_minute_spo2 = ProtoField.uint8("sleepon.minute_data.minute_spo2", "Minute SpO2")
local pf_minute_data_minute_state = ProtoField.uint8("sleepon.minute_data.minute_state", "Minute State")
local pf_minute_data_minute_sport = ProtoField.uint8("sleepon.minute_data.minute_sport", "Minute Sport")

sleepon.fields = { pf_cmd, pf_direction, pf_status, pf_battery_level,
                   pf_heartspo2,
                   pf_heartspo2_spo2, pf_heartspo2_hr, pf_heartspo2_wearing,
                   pf_heartspo2_charging, pf_heartspo2_pi,
                   pf_minute_data, pf_minute_data_num, pf_minute_data_index,
                   pf_minute_data_data_count, pf_minute_data_data_package,
                   pf_minute_data_synchronize, pf_minute_data_period, pf_minute_data_minute_heart,
                   pf_minute_data_minute_spo2, pf_minute_data_minute_state, pf_minute_data_minute_sport,
               }

local fe_direction = Field.new("sleepon.direction")
local fe_cmd = Field.new("sleepon.cmd")

local ef_too_short = ProtoExpert.new("sleepon.too_short", "Packet too short", expert.group.MALFORMED, expert.severity.ERROR)
local ef_extra_data = ProtoExpert.new("sleepon.extra_data", "Extra data", expert.group.UNDECODED, expert.severity.WARN)
sleepon.experts = { ef_too_short, ef_extra_data }

function sleepon.dissector(tvb, pinfo, root)
    pinfo.cols.protocol:set("SleepOn")
    local len = tvb:reported_length_remaining()
    local tree = root:add(sleepon, tvb:range(0, len))
    if len < 2 then
        tree:add_proto_expert_info(ef_too_short)
        return
    end

    tree:add(pf_direction, tvb:range(0, 1))
    local direction = fe_direction().value
    if direction == 0x5a then
        pinfo.cols.protocol:append(" Tx")
    elseif direction == 0x5b then
        pinfo.cols.protocol:append(" Rx")
    end

    tree:add(pf_cmd, tvb:range(1, 1))

    if len < 3 then
        return
    end

    tree:add(pf_status, tvb:range(2, 1))

    local pos = 3
    local cmd = fe_cmd().value

    if cmd == 0x15 then
        if len < 4 then
            tree:add_proto_expert_info(ef_too_short)
            return
        end
        tree:add(pf_battery_level, tvb:range(3, 1))
        pos = pos+1
    elseif cmd == 0x40 then
        tree = tree:add(pf_heartspo2, tvb:range(3))
        if len < 7 then
            tree:add_proto_expert_info(ef_too_short)
            return
        end
        tree:add(pf_heartspo2_spo2, tvb:range(3, 1))
        tree:add(pf_heartspo2_hr, tvb:range(4, 1))
        tree:add(pf_heartspo2_wearing, tvb:range(5, 1))
        tree:add(pf_heartspo2_charging, tvb:range(6, 1))
        pos = pos+4
        if len > 7 then
            tree:add(pf_heartspo2_pi, tvb:range(7, 1))
            pos = pos+1
        end
    elseif cmd == 0x3b then
        if len < 20 then
            tree:add_proto_expert_info(ef_too_short)
            return
        end

        tree = tree:add(pf_minute_data, tvb:range(2))
        tree:add(pf_minute_data_num, tvb:range(2, 2))
        tree:add(pf_minute_data_index, tvb:range(4, 2))
        tree:add(pf_minute_data_data_count, tvb:range(11, 2))
        tree:add(pf_minute_data_data_package, tvb:range(13, 2))
        tree:add(pf_minute_data_synchronize, tvb:range(17, 1))
        tree:add(pf_minute_data_period, tvb:range(7, 4))
        pos = 19
--    local pf_minute_data_minute_heart = ProtoField.uint8("sleepon.minute_data.minute_heart", "Minute Heart")
--    local pf_minute_data_minute_spo2 = ProtoField.uint8("sleepon.minute_data.minute_spo2", "Minute SpO2")
--    local pf_minute_data_minute_state = ProtoField.uint8("sleepon.minute_data.minute_state", "Minute State")
--    local pf_minute_data_minute_sport = ProtoField.uint8("sleepon.minute_data.minute_sport", "Minute Sport")

    end

    if pos < len then
        tree:add_proto_expert_info(ef_extra_data, "Extra data: "..tostring(tvb:range(pos, len-pos)))
    end
end
bt_dissectors = DissectorTable.get("bluetooth.uuid")
bt_dissectors:add(RX_UUID, sleepon)
bt_dissectors:add(TX_UUID, sleepon)
