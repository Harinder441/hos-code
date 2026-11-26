package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	appName    = "hos-code"
	legacyName = "px0"
)

var (
	dirMu       sync.Mutex
	dirResolved           = map[string]string{}
	notesOut    io.Writer = os.Stderr
	envNoted    sync.Map
)

func homeAppDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return appDir(home, "."+appName, "."+legacyName)
}

func xdgAppDir(env string) string {
	base := os.Getenv(env)
	if base == "" {
		return ""
	}
	return appDir(base, appName, legacyName)
}

// The old folder moves once and a link stays at its path, so a px0 binary or a hand-written ~/.px0/hos.env keeps working.
func appDir(base, name, legacy string) string {
	dir := filepath.Join(base, name)
	dirMu.Lock()
	defer dirMu.Unlock()
	if got, ok := dirResolved[dir]; ok {
		return got
	}
	got := migrateAppDir(filepath.Join(base, legacy), dir)
	dirResolved[dir] = got
	return got
}

func migrateAppDir(old, dir string) string {
	if _, err := os.Lstat(dir); err == nil {
		return dir
	}
	info, err := os.Lstat(old)
	if err != nil || !info.IsDir() {
		return dir
	}
	if err := os.Rename(old, dir); err != nil {
		fmt.Fprintf(notesOut, "%s: could not move %s to %s (%v); still using %s\n", appName, old, dir, err, old)
		return old
	}
	if err := os.Symlink(dir, old); err != nil {
		fmt.Fprintf(notesOut, "%s: moved %s to %s\n", appName, old, dir)
	} else {
		fmt.Fprintf(notesOut, "%s: moved %s to %s and linked the old path to it\n", appName, old, dir)
	}
	return dir
}

func getenvCompat(getenv func(string) string, name, old string) string {
	if v := getenv(name); v != "" {
		return v
	}
	v := getenv(old)
	if v != "" {
		if _, seen := envNoted.LoadOrStore(old, true); !seen {
			fmt.Fprintf(notesOut, "%s: %s is deprecated; use %s\n", appName, old, name)
		}
	}
	return v
}
