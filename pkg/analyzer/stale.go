package analyzer

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type StaleFile struct {
	Path        string
	LastModTime time.Time
	DaysIdle    int
}

// FindStaleFiles finds files in the git repo that haven't been modified in more than `idleDays`.
func FindStaleFiles(repoDir string, idleDays int) ([]StaleFile, error) {
	cmd := exec.Command("git", "-C", repoDir, "ls-files")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	files := strings.Split(out.String(), "\n")
	var staleFiles []StaleFile
	threshold := time.Now().AddDate(0, 0, -idleDays)

	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			return nil, nil
		}

		// Get last commit date for this file
		logCmd := exec.Command("git", "-C", repoDir, "log", "-1", "--format=%cd", "--date=iso", "--", file)
		var logOut bytes.Buffer
		logCmd.Stdout = &logOut
		if err := logCmd.Run(); err != nil {
			continue
		}

		dateStr := strings.TrimSpace(logOut.String())
		if dateStr == "" {
			continue
		}

		// Parse git date format: 2026-03-01 14:30:00 +0530
		// Truncate to standard layout prefix
		if len(dateStr) >= 19 {
			t, err := time.Parse("2006-01-02 15:04:05", dateStr[:19])
			if err == nil {
				if t.Before(threshold) {
					days := int(time.Since(t).Hours() / 24)
					staleFiles = append(staleFiles, StaleFile{
						Path:        filepath.ToSlash(file),
						LastModTime: t,
						DaysIdle:    days,
					})
				}
			}
		}
	}

	return staleFiles, nil
}
