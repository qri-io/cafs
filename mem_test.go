package cafs

import (
	"testing"
)

func TestMemFilestore(t *testing.T) {
	ms := NewMapstore()
	if err := RunFilestoreTests(ms); err != nil {
		t.Error(err.Error())
	}
}
