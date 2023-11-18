package location

import "net"

type Location interface {
	City(ip net.IP) (map[string]string, error)
}
