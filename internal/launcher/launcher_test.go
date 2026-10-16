package launcher

import (
	"testing"
)

func TestSubstitute(t *testing.T) {
	got, err := substitute([]string{"code", "{path}"}, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "code" || got[1] != "/repo" {
		t.Fatalf("got %v", got)
	}
}

func TestSubstituteEmbedded(t *testing.T) {
	got, _ := substitute([]string{"term", "--dir={path}"}, "/r")
	if got[1] != "--dir=/r" {
		t.Fatalf("got %v", got)
	}
}

func TestSubstituteNoPlaceholderAppends(t *testing.T) {
	got, _ := substitute([]string{"lazygit"}, "/r")
	if len(got) != 2 || got[1] != "/r" {
		t.Fatalf("got %v", got)
	}
}

func TestSubstituteEmpty(t *testing.T) {
	if _, err := substitute(nil, "/r"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLaunchMissingExecutable(t *testing.T) {
	err := Launch([]string{"definitely-not-a-real-binary-xyz", "{path}"}, "/tmp")
	if err == nil {
		t.Fatal("expected error")
	}
	me, ok := err.(*ErrMissingExecutable)
	if !ok || me.Name != "definitely-not-a-real-binary-xyz" {
		t.Fatalf("got %T %v", err, err)
	}
}
