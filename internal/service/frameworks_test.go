package service

import "testing"

func TestFrameworkItemRepairRulesAreOneToOne(t *testing.T) {
	for frameworkSlug, rules := range frameworkItemRepairRules {
		seenTargets := make(map[string]string, len(rules))
		for _, rule := range rules {
			if existingLegacyCode, ok := seenTargets[rule.CanonicalCode]; ok {
				t.Fatalf("framework %s maps both %s and %s to %s; repair rules must stay one-to-one", frameworkSlug, existingLegacyCode, rule.LegacyCode, rule.CanonicalCode)
			}
			seenTargets[rule.CanonicalCode] = rule.LegacyCode
		}
	}
}
