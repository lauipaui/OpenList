package lsky_pro

import (
	"testing"

	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

func TestUniqueNames(t *testing.T) {
	objs := []model.Obj{
		Image{Key: "aaa", OriginName: "a.png"}.toObj(),
		Image{Key: "bbb", OriginName: "a.png"}.toObj(),
		Image{Key: "ccc", OriginName: "a.png"}.toObj(),
		Image{Key: "ddd", OriginName: "noext"}.toObj(),
		Image{Key: "eee", OriginName: "noext"}.toObj(),
	}
	uniqueNames(objs)
	want := []string{"a.png", "a (bbb).png", "a (ccc).png", "noext", "noext (eee)"}
	for i, o := range objs {
		if o.GetName() != want[i] {
			t.Errorf("objs[%d] = %q, want %q", i, o.GetName(), want[i])
		}
	}
	// ids are untouched, so Link/Remove still hit the right record
	if objs[1].GetID() != "bbb" {
		t.Errorf("id changed: %s", objs[1].GetID())
	}
}
