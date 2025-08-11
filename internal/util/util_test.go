package util

import (
	"testing"
	"time"
)

func TestGetFrontMatter(t *testing.T) {
	testCases := []struct {
		name          string
		markdown      []byte
		expectNil     bool
		expectedTitle string
		expectedDate  time.Time
	}{
		{
			name: "Valid Front Matter",
			markdown: []byte(`%%%
title = "Hello World"
date = 2025-01-01 00:00:00Z
%%%
# Content`),
			expectNil:     false,
			expectedTitle: "Hello World",
			expectedDate:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "No Front Matter",
			markdown: []byte(`# Just Content
No front matter here.`),
			expectNil: true,
		},
		{
			name:      "Empty File",
			markdown:  []byte(""),
			expectNil: true,
		},
		{
			name: "Content Before Front Matter",
			markdown: []byte(`
# This should be ignored
%%%
title = "Hello World"
date = 2025-01-01 00:00:00Z
%%%
# Content`),
			expectNil: true,
		},
		{
			name: "Extra Whitespace",
			markdown: []byte(`


%%%

title = "Hello World"
date = 2025-01-01 00:00:00Z

%%%
# Content`),
			expectNil:     false,
			expectedTitle: "Hello World",
			expectedDate:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Malformed Front Matter",
			markdown: []byte(`%%%
title = "Incomplete
# Content`),
			expectNil: true,
		},
		{
			name: "Front Matter with No Title",
			markdown: []byte(`%%%
date = 2025-01-01 00:00:00Z
%%%
# Content`),
			expectNil:     false,
			expectedTitle: "",
			expectedDate:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Front Matter with No Date",
			markdown: []byte(`%%%
title = "No Date"
%%%
# Content`),
			expectNil:     false,
			expectedTitle: "No Date",
			expectedDate:  time.Time{}, // Zero value for time
		},
		{
			name:          "Only Delimiters",
			markdown:      []byte("%%% %%%"),
			expectNil:     true,
			expectedTitle: "",
			expectedDate:  time.Time{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info := GetFrontMatter(tc.markdown)

			if tc.expectNil {
				if info != nil {
					t.Errorf("Expected nil front matter, but got %+v", info)
				}
				return
			}

			if info == nil {
				t.Fatal("Expected front matter, but got nil")
			}

			if info.Title != tc.expectedTitle {
				t.Errorf("Expected title '%s', but got '%s'", tc.expectedTitle, info.Title)
			}

			if !info.Date.Equal(tc.expectedDate) {
				t.Errorf("Expected date '%v', but got '%v'", tc.expectedDate, info.Date)
			}
		})
	}
}
