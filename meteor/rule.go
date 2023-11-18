package meteor

import (
	"github.com/dushxiiang/meteor/pkg/logger"
	"net"
	"strings"
)

type Rule struct {
	IP      string `yaml:"ip"`
	City    string `yaml:"city"`
	Allowed bool   `yaml:"allowed"`

	ipList   []net.IP
	cidrList []*net.IPNet

	cityList []string
}

func (r *Rule) Init() error {
	parts := strings.Split(r.IP, ",")
	for _, part := range parts {
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			_, cidr, err := net.ParseCIDR(part)
			if err != nil {
				return err
			}
			r.cidrList = append(r.cidrList, cidr)
		} else {
			addr := net.ParseIP(part)
			r.ipList = append(r.ipList, addr)
		}
	}

	if r.City != "" {
		r.cityList = strings.Split(r.City, ",")
	}
	return nil
}

func (r *Rule) MatchIP(x net.IP) bool {
	sugar := logger.L.Sugar()
	for _, addr := range r.ipList {
		b := addr.Equal(x)
		sugar.Debugf("Matching ip: %v equal %v = %v", addr, x, b)
		if b {
			return true
		}
	}
	for _, cidr := range r.cidrList {
		b := cidr.Contains(x)
		sugar.Debugf("Matching cidr: %v contains %v = %v", cidr, x, b)
		if b {
			return true
		}
	}
	return false
}

func (r *Rule) MatchCity(names map[string]string) bool {
	sugar := logger.L.Sugar()
	for _, name := range names {
		for _, city := range r.cityList {
			b := strings.EqualFold(city, name)
			sugar.Debugf("Matching city: %v equal %v = %v", city, name, b)
			if b {
				return true
			}
		}
	}
	return false
}
