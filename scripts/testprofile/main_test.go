package main

import (
	"reflect"
	"testing"
)

func TestSplitGoTestArgs(t *testing.T) {
	patterns, run, flags, err := splitGoTestArgs([]string{"-race", "-count=3", "./internal/web", "./internal/store", "-run", "Plan|Widget", "-benchmem", "-benchtime", "100ms"})
	if err != nil || run != "Plan|Widget" || !reflect.DeepEqual(patterns, []string{"./internal/web", "./internal/store"}) || !reflect.DeepEqual(flags, []string{"-race", "-count=3", "-benchmem", "-benchtime=100ms"}) {
		t.Fatalf("patterns=%v run=%q flags=%v err=%v", patterns, run, flags, err)
	}
}

func TestSplitGoTestArgsRejectsMissingAndUnsupportedFlags(t *testing.T) {
	for _, args := range [][]string{{"-run"}, {"-o", "elsewhere.test"}, {"-cpuprofile=x"}, {"-unknown"}} {
		if _, _, _, err := splitGoTestArgs(args); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
