package service

import "testing"

func TestFrameworkItemRepairMapsAreOneToOne(t *testing.T) {
	for frameworkSlug, repairMap := range frameworkItemRepairMaps {
		seenTargets := make(map[string]string, len(repairMap))
		for oldCode, newCode := range repairMap {
			if existingOldCode, ok := seenTargets[newCode]; ok {
				t.Fatalf("framework %s maps both %s and %s to %s; repair maps must stay one-to-one", frameworkSlug, existingOldCode, oldCode, newCode)
			}
			seenTargets[newCode] = oldCode
		}
	}
}
