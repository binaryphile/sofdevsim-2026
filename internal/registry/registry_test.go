package registry

import "testing"

func TestOfficeSize_DefaultsWhenUnset(t *testing.T) {
	reg := NewSimRegistry()
	w, h := reg.OfficeSize()
	if w != 80 || h != 24 {
		t.Errorf("OfficeSize() = (%d, %d), want (80, 24)", w, h)
	}
}

func TestOfficeSize_ReturnsStoredValues(t *testing.T) {
	reg := NewSimRegistry()
	reg.UpdateOfficeSize(60, 30)
	w, h := reg.OfficeSize()
	if w != 60 || h != 30 {
		t.Errorf("OfficeSize() = (%d, %d), want (60, 30)", w, h)
	}
}
