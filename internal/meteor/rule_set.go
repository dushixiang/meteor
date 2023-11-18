package meteor

import (
	"github.com/dushxiiang/meteor/internal/location"
	"github.com/dushxiiang/meteor/pkg/logger"
	"net"
)

type RuleSet []Rule

func (r RuleSet) Allowed(ip net.IP, ipLocation location.Location) bool {
	sugar := logger.L.Sugar()
	for _, rule := range r {
		if rule.IP != "" {
			if rule.MatchIP(ip) {
				sugar.Debugf("Matching IP succeeded, allowed: %v", rule.Allowed)
				return rule.Allowed
			}
		}

		if rule.City != "" {
			if ipLocation == nil {
				sugar.Warn("Matching city skip, ip location not configed")
				continue
			}
			city, err := ipLocation.City(ip)
			if err != nil {
				sugar.Warnf("Matching city err: %v", err)
				continue
			}
			if rule.MatchCity(city) {
				sugar.Debugf("Matching city succeeded, allowed: %v", rule.Allowed)
				return rule.Allowed
			}
		}
	}
	return true
}
