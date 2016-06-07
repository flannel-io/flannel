// +build windows

// Copyright 2016 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package windows

import (
	"encoding/binary"
	"net"
)

func InetNtoa(ipnr uint32) net.IP {
	bytes := toBytes(ipnr)
	return net.IPv4(bytes[3], bytes[2], bytes[1], bytes[0])
}

func ToIPMask(ipnr uint32) net.IPMask {
	bytes := toBytes(ipnr)
	return net.IPMask(bytes[:])
}

func NetIPToDWORD(ip net.IP) uint32 {
	return binary.LittleEndian.Uint32(ip.To4())
}

func NetIPMaskToDWORD(ip net.IPMask) uint32 {
	return binary.LittleEndian.Uint32(ip)
}

func toBytes(ipnr uint32) [4]byte {
	var bytes [4]byte
	bytes[3] = byte(ipnr & 0xFF)
	bytes[2] = byte((ipnr >> 8) & 0xFF)
	bytes[1] = byte((ipnr >> 16) & 0xFF)
	bytes[0] = byte((ipnr >> 24) & 0xFF)

	return bytes
}
