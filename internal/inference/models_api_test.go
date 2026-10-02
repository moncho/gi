package inference

import "testing"

// The typed catalogs list Pi's image and classifier models; entries carry
// their type and never headers; availability follows credentials.
func TestTypedModelCatalog(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "")
	if len(ModelsOfType("classifier", "")) == 0 || len(ModelsOfType("image", "openrouter")) == 0 {
		t.Fatal("empty typed catalogs")
	}
	img := ModelsOfType("image", "openrouter")[0]
	if img["type"] != "image" || img["headers"] != nil {
		t.Fatalf("entry %v", img)
	}
	id, _ := img["id"].(string)
	if m := ModelOfType("image", "openrouter", id); m == nil || m["id"] != id {
		t.Fatalf("lookup %v", m)
	}
	if ModelOfType("classifier", "openrouter", id) != nil {
		t.Fatal("image model found as classifier")
	}
	if len(AvailableOfType("image", "openrouter")) != 0 {
		t.Fatal("available without credentials")
	}
	t.Setenv("OPENROUTER_API_KEY", "sk-test")
	if len(AvailableOfType("image", "openrouter")) == 0 {
		t.Fatal("not available with OPENROUTER_API_KEY")
	}
}
