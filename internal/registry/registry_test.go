package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAddSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reg.json")
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Entries()) != 0 {
		t.Fatal("expected empty registry")
	}
	r.Add("/a")
	r.Add("/b")
	r.Add("/a") // dedupe
	if err := r.Save(); err != nil {
		t.Fatal(err)
	}

	r2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	es := r2.Entries()
	if len(es) != 2 || es[0].Path != "/a" || es[1].Path != "/b" {
		t.Fatalf("got %+v", es)
	}
}

func TestRemove(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "r.json"))
	r.Add("/a")
	if !r.Remove("/a") || r.Remove("/a") {
		t.Fatal("remove semantics broken")
	}
	if len(r.Entries()) != 0 {
		t.Fatal("expected empty")
	}
}

func TestMarkMissing(t *testing.T) {
	r, _ := Load(filepath.Join(t.TempDir(), "r.json"))
	r.Add("/a")
	r.MarkMissing("/a", true)
	if !r.entries["/a"].Missing {
		t.Fatal("expected missing flag")
	}
	r.MarkMissing("/a", false)
	if r.entries["/a"].Missing {
		t.Fatal("expected missing cleared")
	}
}

func TestCorruptFileRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reg.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(path)
	if err != nil {
		t.Fatalf("corrupt file should recover empty, got %v", err)
	}
	if len(r.Entries()) != 0 {
		t.Fatal("expected empty registry after corruption")
	}
	matches, _ := filepath.Glob(path + ".corrupt.*")
	if len(matches) != 1 {
		t.Fatalf("expected backup file, got %v", matches)
	}
	// A subsequent save works on the fresh registry.
	r.Add("/x")
	if err := r.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var f file
	if err := json.Unmarshal(data, &f); err != nil || len(f.Repos) != 1 {
		t.Fatalf("saved registry unreadable: %v", err)
	}
}

func TestUIStatePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reg.json")
	r, _ := Load(path)
	r.SetUI(UIState{Sort: "name", Filter: "dirty", LastSelected: "/a"})
	r.Save()
	r2, _ := Load(path)
	ui := r2.UI()
	if ui.Sort != "name" || ui.Filter != "dirty" || ui.LastSelected != "/a" {
		t.Fatalf("got %+v", ui)
	}
}
