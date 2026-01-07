package ai

import "testing"

func TestGetReviewAction(t *testing.T) {
	tests := []struct {
		name                 string
		review               ReviewResult
		autoApproveThreshold int
		blockOnCritical      bool
		expected             ReviewAction
	}{
		{
			name: "Critical issue with blocking enabled",
			review: ReviewResult{
				Review:   ReviewSummary{Score: 50},
				Comments: []Comment{{Critical: true}},
			},
			autoApproveThreshold: 90,
			blockOnCritical:      true,
			expected:             ReviewActionRequestChanges,
		},
		{
			name: "Critical issue with blocking disabled",
			review: ReviewResult{
				Review:   ReviewSummary{Score: 50},
				Comments: []Comment{{Critical: true}},
			},
			autoApproveThreshold: 90,
			blockOnCritical:      false,
			expected:             ReviewActionComment,
		},
		{
			name: "High score no comments approves",
			review: ReviewResult{
				Review:   ReviewSummary{Score: 95},
				Comments: []Comment{},
			},
			autoApproveThreshold: 90,
			blockOnCritical:      true,
			expected:             ReviewActionApprove,
		},
		{
			name: "High score with comments stays comment",
			review: ReviewResult{
				Review:   ReviewSummary{Score: 95},
				Comments: []Comment{{Critical: false, Content: "minor issue"}},
			},
			autoApproveThreshold: 90,
			blockOnCritical:      true,
			expected:             ReviewActionComment,
		},
		{
			name: "Low score no comments stays comment",
			review: ReviewResult{
				Review:   ReviewSummary{Score: 70},
				Comments: []Comment{},
			},
			autoApproveThreshold: 90,
			blockOnCritical:      true,
			expected:             ReviewActionComment,
		},
		{
			name: "Exact threshold approves",
			review: ReviewResult{
				Review:   ReviewSummary{Score: 90},
				Comments: []Comment{},
			},
			autoApproveThreshold: 90,
			blockOnCritical:      true,
			expected:             ReviewActionApprove,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := tt.review.GetReviewAction(tt.autoApproveThreshold, tt.blockOnCritical)
			if action != tt.expected {
				t.Errorf("GetReviewAction() = %v, want %v", action, tt.expected)
			}
		})
	}
}
