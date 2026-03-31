package ship

import (
	"testing"
)

func TestParseRemote(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		owner   string
		repo    string
		wantErr bool
	}{
		{"ssh", "git@github.com:owner/repo.git", "owner", "repo", false},
		{"ssh no .git", "git@github.com:owner/repo", "owner", "repo", false},
		{"https", "https://github.com/owner/repo.git", "owner", "repo", false},
		{"https no .git", "https://github.com/owner/repo", "owner", "repo", false},
		{"invalid", "not-a-url", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := parseRemote(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if owner != tt.owner {
				t.Errorf("owner: got %s, want %s", owner, tt.owner)
			}
			if repo != tt.repo {
				t.Errorf("repo: got %s, want %s", repo, tt.repo)
			}
		})
	}
}
