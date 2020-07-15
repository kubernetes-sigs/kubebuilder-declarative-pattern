package loaders

import (
	"testing"
)

func TestParseGitURL(t *testing.T) {
	tests := []struct{
		rawURL string
		baseURL string
		subDir string
	}{
		{
			rawURL: "https://github.com/testRepository.git",
			baseURL: "https://github.com/testRepository.git",
			subDir: "",
		},
		{
			rawURL: "git::https://github.com/testRepository.git",
			baseURL: "https://github.com/testRepository.git",
			subDir: "",
		},
		{
			rawURL: "git::https://github.com/testRepository.git//subDir/package",
			baseURL: "https://github.com/testRepository.git",
			subDir: "subDir/package",
		},
	}

	for _, tt := range tests {
		actualBase, actualSubDir := parseGitURL(tt.rawURL)
		if actualBase != tt.baseURL {
			t.Errorf("Expected base url: %v, got %v", tt.baseURL, actualBase)
		}

		if actualSubDir != tt.subDir {
			t.Errorf("Expected base url: %v, got %v", tt.subDir, actualSubDir)
		}
	}
}
