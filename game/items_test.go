package game

import "testing"

// Unique is the five one-per-planet bonus structures and nothing else.
func TestItemsUnique(t *testing.T) {
	want := map[Item]bool{HyperpowerPlant: true, FertilizationPlant: true, Internet: true, CloningPlant: true, ObservationInstallation: true}
	for it := range NumItems {
		if it.Unique() != want[it] {
			t.Errorf("%s: Unique %v", it, it.Unique())
		}
	}
}
