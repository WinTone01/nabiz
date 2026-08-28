package config

import (
	"encoding/binary"
	"net"
	"strconv"
	"strings"

	"github.com/WinTone01/nabiz/internal/util"
)

// udpListening checks /proc for a UDP socket bound to ip:port without sending
// a single packet - probing a resolver that is not there costs a timeout.
func udpListening(address string) bool {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	wantPort, err := strconv.Atoi(portText)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host).To4()
	if ip == nil {
		return false
	}
	// /proc/net/udp stores the address little-endian, uppercase hex
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint32(raw, binary.BigEndian.Uint32(ip))
	want := strings.ToUpper(hexOf(raw))
	for _, line := range strings.Split(util.ReadText("/proc/net/udp", ""), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		addrHex, portHex, ok := strings.Cut(fields[1], ":")
		if !ok {
			continue
		}
		port, err := strconv.ParseInt(portHex, 16, 32)
		if err != nil || int(port) != wantPort {
			continue
		}
		if addrHex == want || addrHex == "00000000" {
			return true
		}
	}
	return false
}

func hexOf(data []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(data)*2)
	for i, b := range data {
		out[i*2] = digits[b>>4]
		out[i*2+1] = digits[b&0x0f]
	}
	return string(out)
}
