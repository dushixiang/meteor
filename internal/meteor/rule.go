package meteor

import (
	"fmt"
	"log/slog"
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
	r.ipList = nil
	r.cidrList = nil
	r.cityList = nil
	parts := strings.Split(r.IP, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
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
			if addr == nil {
				return fmt.Errorf("invalid IP address %q", part)
			}
			r.ipList = append(r.ipList, addr)
		}
	}

	if r.City != "" {
		for _, city := range strings.Split(r.City, ",") {
			if city = strings.TrimSpace(city); city != "" {
				r.cityList = append(r.cityList, city)
			}
		}
	}
	return nil
}

func (r *Rule) MatchIP(x net.IP) bool {
	for _, addr := range r.ipList {
		b := addr.Equal(x)
		slog.Debug("matching IP", "rule_ip", addr, "client_ip", x, "matched", b)
		if b {
			return true
		}
	}
	for _, cidr := range r.cidrList {
		b := cidr.Contains(x)
		slog.Debug("matching CIDR", "cidr", cidr, "client_ip", x, "matched", b)
		if b {
			return true
		}
	}
	return false
}

func (r *Rule) MatchCity(names map[string]string) bool {
	for _, name := range names {
		for _, city := range r.cityList {
			b := strings.EqualFold(city, name)
			slog.Debug("matching city", "rule_city", city, "city", name, "matched", b)
			if b {
				return true
			}
		}
	}
	return false
}
