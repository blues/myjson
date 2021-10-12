// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Serves Health Checks
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func binDecodeFromTemplate(bin []byte, template string, flagBytes int) (result map[string]interface{}, err error) {

	// Preset result
	result = map[string]interface{}{}

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

	// Generate an array of objects so that we can preserve order.  This is
	// odd, but it's rooted in the fact that maps guarantee random ordering.
	t1 := strings.ReplaceAll(template, "{", "")
	t2 := strings.ReplaceAll(t1, "}", "")
	t3 := strings.ReplaceAll(t2, ":", ",")
	t4 := strings.Split(t3, ",")
	t := t4

	// Iterate over the map
	binOffset := 0
	for i := 0; i < len(t)/2; i++ {

		if binOffset >= len(bin) {
			continue
		}

		k := strings.ReplaceAll(t[i*2], "\"", "")
		v := t[i*2+1]

		isString := false
		isBool := false
		f := float64(0)

		if strings.HasPrefix(v, "\"") {
			f, _ = strconv.ParseFloat(strings.ReplaceAll(v, "\"", ""), 64)
			isString = true
		} else if v == "true" {
			isBool = true
		} else {
			f, _ = strconv.ParseFloat(v, 64)
			if f == 0 {
				continue
			}
		}

		fmt.Printf("OZZIE before: %s,%v %d: %v\n", k, t, binOffset, result)

		if isString {

			strLen := int(f)
			result[k] = binExtractString(bin[binOffset : binOffset+strLen])
			binOffset += strLen

		} else if isBool {

			if (flags & 0x01) != 0 {
				result[k] = true
			} else {
				result[k] = false
			}
			flags = flags >> 1

		} else {

			fmt.Printf("OZZIE: float %f\n", f)

			if isPointOne(f, 12) {
				result[k] = binExtractFloat16(bin[binOffset : binOffset+2])
				binOffset += 2
			} else if isPointOne(f, 14) {
				result[k] = binExtractFloat32(bin[binOffset : binOffset+4])
				binOffset += 4
			} else if isPointOne(f, 18) {
				result[k] = binExtractFloat64(bin[binOffset : binOffset+8])
				binOffset += 8
			} else if f == 11 {
				result[k] = binExtractInt8(bin[binOffset : binOffset+1])
				binOffset++
			} else if f == 12 {
				result[k] = binExtractInt16(bin[binOffset : binOffset+2])
				binOffset += 2
			} else if f == 13 {
				result[k] = binExtractInt24(bin[binOffset : binOffset+3])
				binOffset += 3
			} else if f == 14 {
				result[k] = binExtractInt32(bin[binOffset : binOffset+4])
				binOffset += 4
			} else if f == 18 {
				result[k] = binExtractInt64(bin[binOffset : binOffset+8])
				binOffset += 8
			}

		}

		fmt.Printf("OZZIE after: %s,%v %d: %v\n", k, t, binOffset, result)
	}

	// Done
	fmt.Printf("OZZIE exit: %v\n", result)
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
