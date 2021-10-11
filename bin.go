// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Serves Health Checks
package main

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"strconv"
)

func binDecodeFromTemplate(bin []byte, template map[string]interface{}, flagBytes int) (result map[string]interface{}, err error) {

	// Extract the flags from the end of the bin
	flags := int64(0)
	binLength := len(bin)
	if flagBytes == 1 && binLength >= 1 {
		flags = int64(binExtractInt8(bin[binLength-1 : binLength]))
	} else if flagBytes == 2 && binLength >= 2 {
		flags = int64(binExtractInt16(bin[binLength-2 : binLength]))
	} else if flagBytes == 4 && binLength >= 4 {
		flags = int64(binExtractInt32(bin[binLength-4 : binLength]))
	} else if flagBytes == 8 && binLength >= 8 {
		flags = binExtractInt64(bin[binLength-8 : binLength])
	}

	// Iterate over the map
	binOffset := 0
	for k, t := range template {

		// Behave differently based on type
		switch t.(type) {

		case string:
			strLen := len(t.(string))
			i, err2 := strconv.Atoi(t.(string))
			if err2 == nil && i > 0 {
				strLen = i
			}
			result[k] = binExtractString(bin[binOffset : binOffset+strLen])
			binOffset += strLen

		case json.Number:
			numberType, errInt := t.(json.Number).Int64()
			if errInt == nil {
				// Integer
				switch numberType {
				case 11:
					result[k] = binExtractInt8(bin[binOffset : binOffset+1])
					binOffset++
				case 12:
					result[k] = binExtractInt16(bin[binOffset : binOffset+2])
					binOffset += 2
				case 13:
					result[k] = binExtractInt24(bin[binOffset : binOffset+3])
					binOffset += 3
				case 14:
					result[k] = binExtractInt32(bin[binOffset : binOffset+4])
					binOffset += 4
				case 18:
					result[k] = binExtractInt64(bin[binOffset : binOffset+8])
					binOffset += 8
				}
			} else {
				numberType, errFloat := t.(json.Number).Float64()
				if errFloat == nil {
					// Real
					if isPointOne(numberType, 12) {
						result[k] = binExtractFloat16(bin[binOffset : binOffset+2])
						binOffset += 2
					} else if isPointOne(numberType, 14) {
						result[k] = binExtractFloat32(bin[binOffset : binOffset+4])
						binOffset += 4
					} else if isPointOne(numberType, 18) || isPointOne(numberType, 1) {
						result[k] = binExtractFloat64(bin[binOffset : binOffset+8])
						binOffset += 8
					}
				}
			}
		case bool:
			if (flags & 0x01) != 0 {
				result[k] = true
			} else {
				result[k] = false
			}
			flags = flags >> 1
		}
	}

	// Done
	return

}

// See if a floating value is ".1" - that is, between N.0 and N.2
func isPointOne(test float64, base float64) bool {
	return test > base && test < base+0.2
}

// Data extraction routines
func binExtractInt8(bin []byte) int8 {
	return int8(bin[0])
}
func binExtractInt16(bin []byte) int16 {
	var value int16
	value = int16(bin[0])
	value = value | (int16(bin[1]) << 8)
	return value
}
func binExtractInt24(bin []byte) int32 {
	value := int32(bin[0])
	value = value | (int32(bin[1]) << 8)
	msb := int8(bin[2])
	msbSignExtended := int32(msb)
	value = value | (msbSignExtended << 16)
	return value
}
func binExtractInt32(bin []byte) int32 {
	var value int32
	value = int32(bin[0])
	value = value | (int32(bin[1]) << 8)
	value = value | (int32(bin[2]) << 16)
	value = value | (int32(bin[3]) << 24)
	return value
}
func binExtractInt64(bin []byte) int64 {
	var value int64
	value = int64(bin[0])
	value = value | (int64(bin[1]) << 8)
	value = value | (int64(bin[2]) << 16)
	value = value | (int64(bin[3]) << 24)
	value = value | (int64(bin[4]) << 32)
	value = value | (int64(bin[5]) << 40)
	value = value | (int64(bin[6]) << 48)
	value = value | (int64(bin[7]) << 56)
	return value
}
func binExtractString(bin []byte) string {
	s := ""
	for i := 0; i < len(bin); i++ {
		if bin[i] != 0 {
			s += string(bin[i])
		}
	}
	return s
}
func binExtractFloat16(bin []byte) float32 {
	value := uint16(bin[0])
	value = value | (uint16(bin[1]) << 8)
	f16 := Float16(value)
	return f16.Float32()
}
func binExtractFloat32(bin []byte) float32 {
	bits := binary.LittleEndian.Uint32(bin)
	return math.Float32frombits(bits)
}
func binExtractFloat64(bin []byte) float64 {
	bits := binary.LittleEndian.Uint64(bin)
	return math.Float64frombits(bits)
}
func binExtractBytes(bin []byte) []byte {
	return bin
}
