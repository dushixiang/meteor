package meteor

import (
	"net"
	"testing"
)

func TestRuleWhitespaceAndReinitialization(t *testing.T) {
	rule := Rule{IP: " 127.0.0.1 , 10.0.0.0/8 ", City: " Beijing , 成都 "}
	if err := rule.Init(); err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"127.0.0.1", "10.1.2.3"} {
		if !rule.MatchIP(net.ParseIP(ip)) {
			t.Fatalf("did not match %s", ip)
		}
	}
	if !rule.MatchCity(map[string]string{"en": "beijing"}) || !rule.MatchCity(map[string]string{"zh-CN": "成都"}) {
		t.Fatal("city whitespace not trimmed")
	}
	rule.IP = "192.0.2.1"
	rule.City = ""
	if err := rule.Init(); err != nil {
		t.Fatal(err)
	}
	if rule.MatchIP(net.ParseIP("127.0.0.1")) || rule.MatchCity(map[string]string{"en": "Beijing"}) {
		t.Fatal("old rules retained after reinitialization")
	}
}

type countingLocation struct{ calls int }

func (c *countingLocation) City(net.IP) (map[string]string, error) {
	c.calls++
	return map[string]string{"en": "Chengdu"}, nil
}

func TestRuleSetOrderedAndSingleGeoIPLookup(t *testing.T) {
	rules := RuleSet{{City: "Beijing", Allowed: false}, {City: "Chengdu", Allowed: true}, {IP: "0.0.0.0/0", Allowed: false}}
	for i := range rules {
		if err := rules[i].Init(); err != nil {
			t.Fatal(err)
		}
	}
	locator := &countingLocation{}
	if !rules.Allowed(net.ParseIP("192.0.2.1"), locator) {
		t.Fatal("first matching city rule ignored")
	}
	if locator.calls != 1 {
		t.Fatalf("looked up city %d times", locator.calls)
	}
}
