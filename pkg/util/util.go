package util

import (
	"crypto/rand"
	"fmt"
)

// generate a random mac address
func RandomMac() (string, error) {
	buf := make([]byte, 3)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	// this mac doesnt have an OUI, so zero out the most significant 3 bytes
	mac := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", 0x00, 0x00, 0x00, buf[0], buf[1], buf[2])
	return mac, nil
}
