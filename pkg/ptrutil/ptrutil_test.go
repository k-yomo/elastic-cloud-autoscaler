package ptrutil

import (
	"reflect"
	"testing"
)

func TestToPtr(t *testing.T) {
	t.Parallel()

	v := 1
	want := &v
	if got := ToPtr(v); !reflect.DeepEqual(got, want) {
		t.Errorf("ToPtr() = %v, want %v", got, want)
	}
}
