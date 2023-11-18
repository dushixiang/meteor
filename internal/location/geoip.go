package location

import (
	"github.com/oschwald/geoip2-golang"
	"net"
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
	city, err := r.ipdb.City(ip)
	if err != nil {
		return nil, err
	}
	return city.City.Names, err
}

var _ Location = (*geoIPLocation)(nil)
