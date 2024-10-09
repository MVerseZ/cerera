package network

import (
	"log"
	"net"
)

func CheckIPAddressType(ip string) int {
	if net.ParseIP(ip) == nil {
		log.Printf("Invalid IP Address: %s\n", ip)
		return 1
	}
	for i := 0; i < len(ip); i++ {
		switch ip[i] {
		case '.':
			log.Printf("Given IP Address %s is IPV4 type\n", ip)
			return 2
		case ':':
			log.Printf("Given IP Address %s is IPV6 type\n", ip)
			return 3
		}
	}
	return 4
}
