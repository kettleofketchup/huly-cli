package skills

import "testing"

func TestCatalogIntegrity(t *testing.T) {
	cat, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) == 0 {
		t.Fatal("catalog is empty")
	}
	for _, s := range cat {
		if s.Name == "" {
			t.Errorf("skill at %s has empty frontmatter name", s.fsPath)
		}
		if s.Description == "" {
			t.Errorf("skill %q has empty description", s.Name)
		}
		// Directory name must match frontmatter name.
		if got := s.fsPath; got != "assets/"+s.Name {
			t.Errorf("dir %q does not match frontmatter name %q", got, s.Name)
		}
	}
	if _, ok := Get("huly-issue-tracking"); !ok {
		t.Error("Get(huly-issue-tracking) not found")
	}
	if _, ok := Get("does-not-exist"); ok {
		t.Error("Get(does-not-exist) should be false")
	}
}
