package meteor

import (
	"github.com/dushxiiang/meteor/internal/location"
	"log/slog"
	"net"
)

type RuleSet []Rule

func (r RuleSet) Allowed(ip net.IP, ipLocation location.Location) bool {
	var city map[string]string
	var lookupErr error
	var lookedUp bool
	for _, rule := range r {
		if rule.IP != "" {
			if rule.MatchIP(ip) {
				slog.Debug("IP rule matched", "client_ip", ip, "allowed", rule.Allowed)
				return rule.Allowed
			}
		}

		if rule.City != "" {
			if ipLocation == nil {
				slog.Warn("city rule skipped: location not configured", "client_ip", ip)
				continue
			}
			if !lookedUp {
				city, lookupErr = ipLocation.City(ip)
				lookedUp = true
			}
			if lookupErr != nil {
				slog.Warn("city lookup failed", "client_ip", ip, "error", lookupErr)
				continue
			}
			if rule.MatchCity(city) {
				slog.Debug("city rule matched", "client_ip", ip, "allowed", rule.Allowed)
				return rule.Allowed
			}
		}
	}
	return true
}
