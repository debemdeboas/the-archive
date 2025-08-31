package util

import (
	"testing"
	"time"
)

func TestGetFrontMatter(t *testing.T) {
	testCases := []struct {
		name          string
		markdown      []byte
		expectError   bool
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
			expectError:   false,
			expectedTitle: "Hello World",
			expectedDate:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "No Front Matter",
			markdown: []byte(`# Just Content
No front matter here.`),
			expectError: true,
		},
		{
			name:        "Empty File",
			markdown:    []byte(""),
			expectError: true,
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
			expectError: true,
		},
		{
			name: "Extra Whitespace",
			markdown: []byte(`


%%%

title = "Hello World"
date = 2025-01-01 00:00:00Z

%%%
# Content`),
			expectError:   false,
			expectedTitle: "Hello World",
			expectedDate:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Malformed Front Matter",
			markdown: []byte(`%%%
title = "Incomplete
# Content`),
			expectError: true,
		},
		{
			name: "Front Matter with No Title",
			markdown: []byte(`%%%
date = 2025-01-01 00:00:00Z
%%%
# Content`),
			expectError:   false,
			expectedTitle: "",
			expectedDate:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Front Matter with No Date",
			markdown: []byte(`%%%
title = "No Date"
%%%
# Content`),
			expectError:   false,
			expectedTitle: "No Date",
			expectedDate:  time.Time{}, // Zero value for time
		},
		{
			name:        "Only Delimiters",
			markdown:    []byte("%%% %%%"),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := GetFrontMatter(tc.markdown)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
				if info != nil {
					t.Errorf("Expected nil info when error occurs, but got %+v", info)
				}
				return
			}

			if err != nil {
				t.Fatalf("Expected no error, but got: %v", err)
			}

			if info == nil {
				t.Fatal("Expected front matter info, but got nil")
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
