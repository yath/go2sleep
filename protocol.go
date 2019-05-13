package main

import "fmt"

type heartSpO2Data struct {
	spO2      byte // SpO₂ %
	heartRate byte // Heart rate, 1/min
	wearing   bool
	charging  byte // 0=no, 1=yes, 2=full?
	pi        byte
}

type batteryData struct {
	level byte // percent
}

type versionData struct {
	version string
}

type unknownData []byte

func unmarshalData(data []byte) (interface{}, error) {
	if len(data) < 2 || data[0] != 0x5b {
		return nil, fmt.Errorf("too short or first byte not 0x5b: %#v", data)
	}

	switch data[1] {
	case 0x40:
		if len(data) < 7 {
			return nil, fmt.Errorf("heartSpO2Data too short, want 7 or 8 bytes: %#v", data)
		}
		var pi byte
		if len(data) > 7 {
			pi = data[7]
		}
		wearing := false
		if data[5] > 0 {
			wearing = true
		}
		return &heartSpO2Data{data[3], data[4], wearing, data[6], pi}, nil

	case 0x14:
		if len(data) < 4 {
			return nil, fmt.Errorf("versionData too short, want >=4 bytes: %#v", data)
		}
		return &versionData{string(data[3:])}, nil

	case 0x15:
		if len(data) < 3 {
			return nil, fmt.Errorf("batteryData too short, want 3 bytes: %#v", data)
		}
		return &batteryData{data[3]}, nil

	default:
		return unknownData(data), nil
	}
}
