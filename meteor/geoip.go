package meteor

import (
	"errors"
	"github.com/dushxiiang/meteor/pkg/logger"
	"go.uber.org/zap"
	"net"

	"github.com/oschwald/geoip2-golang"
)

var ipDB *geoip2.Reader

func InitGeoIP(file string) (err error) {
	ipDB, err = geoip2.Open(file)
	if err != nil {
		return err
	}
	logger.L.Debug("init geoip succeeded", zap.Any("metadata", ipDB.Metadata()))
	return nil
}

func City(ip net.IP) (*geoip2.City, error) {
	if ipDB == nil {
		return nil, errors.New("not config geoip file")
	}
	return ipDB.City(ip)
}
