package skill

import "testing"

func TestParseInline(t *testing.T) {
	src := "---\nname: foo-bar\ndescription: A short one.\nmetadata: { axis: proc, origin: \"x\" }\n---\n# Body\ntext\n"
	s := Parse("foo-bar", []byte(src))
	if !s.HasFront {
		t.Fatal("expected HasFront")
	}
	if s.FrontName != "foo-bar" {
		t.Errorf("FrontName = %q", s.FrontName)
	}
	if s.Description != "A short one." {
		t.Errorf("Description = %q", s.Description)
	}
	if s.Axis != "proc" {
		t.Errorf("Axis = %q", s.Axis)
	}
	if s.Name != "foo-bar" {
		t.Errorf("Name = %q", s.Name)
	}
}

func TestParseFoldedDescription(t *testing.T) {
	src := "---\nname: lang-go-x\ndescription: >\n  First line\n  second line.\nmetadata: { axis: lang-go }\n---\nbody here\n"
	s := Parse("lang-go-x", []byte(src))
	if s.Description != "First line second line." {
		t.Errorf("folded Description = %q", s.Description)
	}
	if s.Axis != "lang-go" {
		t.Errorf("Axis = %q", s.Axis)
	}
	if s.Body == "" || s.Body[:9] != "body here" {
		t.Errorf("Body = %q", s.Body)
	}
}

func TestParseQuotedName(t *testing.T) {
	src := "---\nname: 'quoted-name'\ndescription: \"a desc value\"\n---\n"
	s := Parse("quoted-name", []byte(src))
	if s.FrontName != "quoted-name" {
		t.Errorf("FrontName = %q", s.FrontName)
	}
	if s.Description != "a desc value" {
		t.Errorf("Description = %q", s.Description)
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	src := "no fence here\njust text\n"
	s := Parse("x", []byte(src))
	if s.HasFront {
		t.Error("expected HasFront == false")
	}
	if s.Body != src {
		t.Errorf("Body = %q, want whole content", s.Body)
	}
	if s.Axis != "" {
		t.Errorf("Axis should be empty when no frontmatter, got %q", s.Axis)
	}
}

func TestParseUnterminated(t *testing.T) {
	s := Parse("x", []byte("---\nname: x\nno closing fence\n"))
	if s.HasFront {
		t.Error("unterminated frontmatter must yield HasFront == false")
	}
}

func TestHashStableAndDistinct(t *testing.T) {
	a := Parse("a", []byte("same"))
	b := Parse("b", []byte("same"))
	c := Parse("c", []byte("different"))
	if a.Hash != b.Hash {
		t.Error("equal content must hash equally")
	}
	if a.Hash == c.Hash {
		t.Error("different content must hash differently")
	}
	if len(a.Hash) != 64 {
		t.Errorf("sha256 hex should be 64 chars, got %d", len(a.Hash))
	}
}

func TestIsKebab(t *testing.T) {
	for _, ok := range []string{"a", "lang-java", "proc-conventional-commits", "x9-y2"} {
		if !IsKebab(ok) {
			t.Errorf("%q should be kebab", ok)
		}
	}
	for _, bad := range []string{"", "Foo", "a_b", "-a", "a-", "a--b", "a b"} {
		if IsKebab(bad) {
			t.Errorf("%q should not be kebab", bad)
		}
	}
}
