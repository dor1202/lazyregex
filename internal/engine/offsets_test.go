package engine

import (
	"reflect"
	"testing"
)

func TestToByteOffsetsAllUnits(t *testing.T) {
	text := "héllo 😀 wörld"
	cases := []struct {
		name string
		unit Unit
		in   []int
		want []int
	}{
		{"codepoints", UnitCodePoints, []int{0, 1, 6, 7, 8}, []int{0, 1, 7, 11, 12}},
		{"utf16", UnitUTF16, []int{0, 1, 6, 8, 9}, []int{0, 1, 7, 11, 12}},
		{"bytes", UnitBytes, []int{0, 1, 7, 11, 12}, []int{0, 1, 7, 11, 12}},
	}
	for _, c := range cases {
		if got := ToByteOffsets(text, c.unit, c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestToByteOffsetsEnd(t *testing.T) {
	text := "héllo 😀 wörld"
	if got := ToByteOffsets(text, UnitCodePoints, []int{13}); got[0] != len(text) {
		t.Errorf("codepoint end: %v", got)
	}
	if got := ToByteOffsets(text, UnitUTF16, []int{14}); got[0] != len(text) {
		t.Errorf("utf16 end: %v", got)
	}
}
