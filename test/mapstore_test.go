package test

import (
	"testing"

	"github.com/qri-io/cafs"
)

func TestMemFilestore(t *testing.T) {
	ms := cafs.NewMapstore()
	if err := EnsureFilestoreBehavior(ms); err != nil {
		t.Error(err.Error())
	}
	// TODO (b5): this is broken :/ fix. think the problem is qfs.MakeDirP
	// if err := EnsureDirectoryBehavior(ms); err != nil {
	// 	t.Error(err.Error())
	// }
}

func TestPathPrefix(t *testing.T) {
	got := cafs.NewMapstore().PathPrefix()
	if "map" != got {
		t.Errorf("path prefix mismatch. expected: 'map', got: %s", got)
	}
}
