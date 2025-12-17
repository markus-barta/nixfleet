package github

import "testing"

func TestPullRequest_IsFlakeLockUpdate(t *testing.T) {
	tests := []struct {
		name   string
		pr     PullRequest
		expect bool
	}{
		{
			name: "has automated label",
			pr: PullRequest{
				Title:  "Some PR",
				Labels: []Label{{Name: "automated"}},
			},
			expect: true,
		},
		{
			name: "has dependencies label",
			pr: PullRequest{
				Title:  "Bump something",
				Labels: []Label{{Name: "dependencies"}},
			},
			expect: true,
		},
		{
			name: "title contains flake.lock",
			pr: PullRequest{
				Title:  "Update flake.lock",
				Labels: []Label{},
			},
			expect: true,
		},
		{
			name: "title contains Update flake",
			pr: PullRequest{
				Title:  "Update flake inputs",
				Labels: []Label{},
			},
			expect: true,
		},
		{
			name: "case insensitive title",
			pr: PullRequest{
				Title:  "FLAKE.LOCK update",
				Labels: []Label{},
			},
			expect: true,
		},
		{
			name: "unrelated PR",
			pr: PullRequest{
				Title:  "Add new feature",
				Labels: []Label{{Name: "feature"}},
			},
			expect: false,
		},
		{
			name: "empty PR",
			pr: PullRequest{
				Title:  "",
				Labels: []Label{},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pr.IsFlakeLockUpdate()
			if got != tt.expect {
				t.Errorf("IsFlakeLockUpdate() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestPullRequest_IsMergeable(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name   string
		pr     PullRequest
		expect bool
	}{
		{
			name: "open and mergeable",
			pr: PullRequest{
				State:          "open",
				Merged:         false,
				Mergeable:      &trueVal,
				MergeableState: "mergeable",
			},
			expect: true,
		},
		{
			name: "open with unknown mergeable state",
			pr: PullRequest{
				State:          "open",
				Merged:         false,
				Mergeable:      nil,
				MergeableState: "",
			},
			expect: true,
		},
		{
			name: "closed PR",
			pr: PullRequest{
				State:          "closed",
				Merged:         false,
				MergeableState: "mergeable",
			},
			expect: false,
		},
		{
			name: "already merged",
			pr: PullRequest{
				State:          "open",
				Merged:         true,
				MergeableState: "mergeable",
			},
			expect: false,
		},
		{
			name: "not mergeable",
			pr: PullRequest{
				State:          "open",
				Merged:         false,
				Mergeable:      &falseVal,
				MergeableState: "conflicting",
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pr.IsMergeable()
			if got != tt.expect {
				t.Errorf("IsMergeable() = %v, want %v", got, tt.expect)
			}
		})
	}
}

