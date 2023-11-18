package meteor

import (
	"github.com/dushxiiang/meteor/pkg/logger"
	"net"
)

type RuleSet []Rule

func (r RuleSet) Allowed(ip net.IP) bool {
	sugar := logger.L.Sugar()
	for _, rule := range r {
		if rule.IP != "" {
			if rule.MatchIP(ip) {
				sugar.Debugf("Matching IP succeeded, allowed: %v", rule.Allowed)
				return rule.Allowed
			}
		}

		if rule.City != "" {
			city, err := City(ip)
			if err != nil {
				sugar.Warnf("Matching city skip, message: %v", err)
				continue
			}
			if rule.MatchCity(city.City.Names) {
				sugar.Debugf("Matching city succeeded, allowed: %v", rule.Allowed)
				return rule.Allowed
			}
		}
	}
	return true
}
