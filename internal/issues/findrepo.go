package issues

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindRepo walks up from the process cwd until a directory contains a
// VCS marker. With "auto", .jj wins over .git at each level; an
// explicit "jj" or "git" restricts detection to that marker.
func FindRepo(vcs string) (string, error) {
	return findRepoOverride(".", vcs)
}

func findRepoOverride(start, vcs string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		marker := ""
		switch vcs {
		case "auto":
			switch {
			case exists(filepath.Join(dir, ".jj")):
				marker = ".jj"
			case exists(filepath.Join(dir, ".git")):
				marker = ".git"
			}
		case "jj":
			if exists(filepath.Join(dir, ".jj")) {
				marker = ".jj"
			}
		case "git":
			if exists(filepath.Join(dir, ".git")) {
				marker = ".git"
			}
		default:
			return "", fmt.Errorf("issues: unknown --vcs value %q", vcs)
		}
		if marker != "" {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("issues: no repo root (.jj or .git) found from %s", start)
		}
		dir = parent
	}
}

func findRepoFrom(start string) (string, error) {
	return findRepoOverride(start, "auto")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
