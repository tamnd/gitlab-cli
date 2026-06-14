package gitlab

import (
	"testing"
)

// These tests exercise the URI driver's pure string functions — no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "gitlab" {
		t.Errorf("Scheme = %q, want gitlab", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "lab" {
		t.Errorf("Identity.Binary = %q, want lab", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in, typ, id string
	}{
		{"gitlab-org/gitlab", "project", "gitlab-org/gitlab"},
		{"https://gitlab.com/gitlab-org/gitlab", "project", "gitlab-org/gitlab"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify(\"\") should return error")
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("project", "gitlab-org/gitlab")
	want := "https://gitlab.com/gitlab-org/gitlab"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("bogus", "x")
	if err == nil {
		t.Error("Locate with unknown type should return error")
	}
}
