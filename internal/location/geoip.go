package location

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/oschwald/geoip2-golang/v2"
)

func NewGeoIPLocation(file string) (Location, error) {
	ipdb, err := geoip2.Open(file)
	if err != nil {
		return nil, err
	}
	location := geoIPLocation{
		ipdb: ipdb,
	}
	return &location, nil
}

type geoIPLocation struct {
	ipdb *geoip2.Reader
}

func (r geoIPLocation) City(ip net.IP) (map[string]string, error) {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return nil, fmt.Errorf("invalid IP address %v", ip)
	}
	city, err := r.ipdb.City(addr.Unmap())
	if err != nil {
		return nil, err
	}
	names := city.City.Names
	return map[string]string{"de": names.German, "en": names.English, "es": names.Spanish, "fr": names.French, "ja": names.Japanese, "pt-BR": names.BrazilianPortuguese, "ru": names.Russian, "zh-CN": names.SimplifiedChinese}, nil
}

var _ Location = (*geoIPLocation)(nil)

func (r *geoIPLocation) Close() error { return r.ipdb.Close() }
