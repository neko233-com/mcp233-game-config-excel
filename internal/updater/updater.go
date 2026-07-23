// Package updater manages offline MCP binary selection without network access.
package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	binaryPrefix = "mcp233-game-config-excel-v"
	binarySuffix = ".exe"
	activeFile   = "mcp233-game-config-excel.active-version.txt"
)

// Version is a parsed numeric MCP binary version.
type Version struct {
	Raw   string `json:"raw"`
	Parts []int  `json:"parts"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
}

// Status describes local offline update candidates and current automatic selection.
type Status struct {
	Directory       string    `json:"directory"`
	ActivePolicy    string    `json:"activePolicy"`
	SelectedVersion string    `json:"selectedVersion"`
	SelectedPath    string    `json:"selectedPath"`
	Versions        []Version `json:"versions"`
	NetworkDisabled bool      `json:"networkDisabled"`
}

// ReadStatus finds validated local versioned binaries. It never contacts a network service.
func ReadStatus(directory string) (Status, error) {
	directory = filepath.Clean(directory)
	versions, err := findVersions(directory)
	if err != nil {
		return Status{}, err
	}
	policy, err := readPolicy(directory)
	if err != nil {
		return Status{}, err
	}
	selected, err := SelectVersion(versions, policy)
	if err != nil {
		return Status{}, err
	}
	return Status{Directory: directory, ActivePolicy: policy, SelectedVersion: selected.Raw, SelectedPath: selected.Path, Versions: versions, NetworkDisabled: true}, nil
}

// SelectVersion chooses highest version for auto, otherwise exact configured version.
func SelectVersion(versions []Version, policy string) (Version, error) {
	if len(versions) == 0 {
		return Version{}, fmt.Errorf("no local versioned MCP binaries found")
	}
	if policy == "" || policy == "auto" {
		return versions[len(versions)-1], nil
	}
	for index := 0; index < len(versions); index++ {
		if versions[index].Raw == policy {
			return versions[index], nil
		}
	}
	return Version{}, fmt.Errorf("configured MCP version not found locally: %s", policy)
}

// SetActiveVersion previews or writes the selected local version policy. version=auto enables automatic newest-version selection.
func SetActiveVersion(directory, version string, apply bool) (Status, error) {
	status, err := ReadStatus(directory)
	if err != nil {
		return Status{}, err
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return Status{}, fmt.Errorf("version is required")
	}
	if _, err := SelectVersion(status.Versions, version); err != nil {
		return Status{}, err
	}
	status.ActivePolicy = version
	selected, err := SelectVersion(status.Versions, version)
	if err != nil {
		return Status{}, err
	}
	status.SelectedVersion = selected.Raw
	status.SelectedPath = selected.Path
	if !apply {
		return status, nil
	}
	if err := os.WriteFile(filepath.Join(status.Directory, activeFile), []byte(version+"\n"), 0o644); err != nil {
		return Status{}, fmt.Errorf("write active version: %w", err)
	}
	return status, nil
}

// Rollback chooses the version before current selection, preserving all binaries.
func Rollback(directory string, apply bool) (Status, error) {
	status, err := ReadStatus(directory)
	if err != nil {
		return Status{}, err
	}
	selectedIndex := -1
	for index := 0; index < len(status.Versions); index++ {
		if status.Versions[index].Raw == status.SelectedVersion {
			selectedIndex = index
			break
		}
	}
	if selectedIndex <= 0 {
		return Status{}, fmt.Errorf("no older local MCP version available for rollback")
	}
	return SetActiveVersion(directory, status.Versions[selectedIndex-1].Raw, apply)
}

func findVersions(directory string) ([]Version, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read MCP binary directory: %w", err)
	}
	versions := make([]Version, 0)
	for index := 0; index < len(entries); index++ {
		entry := entries[index]
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), binaryPrefix) || !strings.HasSuffix(strings.ToLower(entry.Name()), binarySuffix) {
			continue
		}
		raw := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), binaryPrefix), binarySuffix)
		parts, parseErr := parseVersion(raw)
		if parseErr != nil {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || info.Size() == 0 {
			continue
		}
		versions = append(versions, Version{Raw: raw, Parts: parts, Path: filepath.Join(directory, entry.Name()), Size: info.Size()})
	}
	sort.Slice(versions, func(left, right int) bool { return compareVersions(versions[left].Parts, versions[right].Parts) < 0 })
	return versions, nil
}

func readPolicy(directory string) (string, error) {
	content, err := os.ReadFile(filepath.Join(directory, activeFile))
	if os.IsNotExist(err) {
		return "auto", nil
	}
	if err != nil {
		return "", err
	}
	policy := strings.TrimSpace(string(content))
	if policy == "" {
		return "auto", nil
	}
	return policy, nil
}

func parseVersion(raw string) ([]int, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("version must use X.Y.Z")
	}
	result := make([]int, 0, len(parts))
	for index := 0; index < len(parts); index++ {
		value, err := strconv.Atoi(parts[index])
		if err != nil || value < 0 {
			return nil, fmt.Errorf("invalid version")
		}
		result = append(result, value)
	}
	return result, nil
}

func compareVersions(left, right []int) int {
	length := len(left)
	if len(right) > length {
		length = len(right)
	}
	for index := 0; index < length; index++ {
		leftValue := 0
		rightValue := 0
		if index < len(left) {
			leftValue = left[index]
		}
		if index < len(right) {
			rightValue = right[index]
		}
		if leftValue < rightValue {
			return -1
		}
		if leftValue > rightValue {
			return 1
		}
	}
	return 0
}
