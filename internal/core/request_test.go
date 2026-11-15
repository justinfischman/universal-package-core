package core

import (
	"reflect"
	"testing"
)

func TestAllGroupsMergesGroupAndGroups(t *testing.T) {
	r := Request{Group: "dev", Groups: []string{"", "docs", "dev"}}
	if got := r.AllGroups(); !reflect.DeepEqual(got, []string{"dev", "docs"}) {
		t.Errorf("AllGroups = %v", got)
	}
	if got := (Request{}).AllGroups(); len(got) != 0 {
		t.Errorf("empty AllGroups = %v", got)
	}
}
