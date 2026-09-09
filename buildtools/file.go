// Copyright 2025 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package buildtools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MkdirAll returns a Step that creates a directory and all necessary parents using mkdir -p.
func MkdirAll(d string) Step {
	if d == "" {
		return ErrorStep(fmt.Errorf("cannot create directory with empty name"), "mkdir", "-p")
	}
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "mkdir", "-p", d)
	})
}

// RmdirAll returns a Step that removes an app bundle and all its contents using rm -rf.
func RmdirAll(d string) Step {
	d = filepath.Clean(d)
	if !strings.HasSuffix(d, ".app") {
		return ErrorStep(fmt.Errorf("can only remove .app directories"), "rm", "-rf", d)
	}
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		if _, err := os.Stat(d); err != nil {
			if os.IsNotExist(err) {
				return NewStepResult("directory does not exist, skipping removal", nil, nil, nil), nil
			}
			return ErrorStep(fmt.Errorf("error checking directory: %v", err), "rm", "-rf", d).Run(ctx, cmdRunner)
		}
		return cmdRunner.Run(ctx, "rm", "-rf", d)
	})
}

// ChmodAll returns a Step that recursively changes the permissions of a directory and its contents using chmod -R.
func ChmodAll(d string, perm os.FileMode) Step {
	d = filepath.Clean(d)
	if !strings.HasSuffix(d, ".app") {
		return ErrorStep(fmt.Errorf("can only chmod .app directories"), "chmod", "-R", fmt.Sprintf("%o", perm), d)
	}
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		if _, err := os.Stat(d); err != nil {
			if os.IsNotExist(err) {
				return NewStepResult("directory does not exist, skipping chmod", nil, nil, nil), nil
			}
			return ErrorStep(fmt.Errorf("error checking directory: %v", err), "chmod", "-R", fmt.Sprintf("%o", perm), d).Run(ctx, cmdRunner)
		}
		return cmdRunner.Run(ctx, "chmod", "-R", fmt.Sprintf("%o", perm), d)
	})
}

// DirExists returns a Step that checks for the existence of the directory.
func DirExists(d string) Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "test", "-d", d)
	})
}

// FileExists returns a Step that checks for the existence of the file.
func FileExists(f string) Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "test", "-f", f)
	})
}

// IconSetDir represents a directory for an icon set.
type IconSetDir string

// IsValidIsValidIconSetDir returns a Step that checks if the directory has a .iconset extension.
func IsValidIconSetDir(id IconSetDir) Step {
	p := string(id)
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return Suffix(".iconset").Assert(p).Run(ctx, cmdRunner)
	})
}

// Rename returns a Step that renames a file using mv.
func Rename(oldname, newname string) Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "mv", oldname, newname)
	})
}

// Copy returns a Step that copies a file using cp.
func Copy(oldname, newname string) Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "cp", oldname, newname)
	})
}

// chmodBits returns the traditional UNIX permission, setuid, setgid, and sticky
// bits from perm formatted for chmod, masking out any file type bits (such as
// os.ModeDir or os.ModeSymlink).
func chmodBits(perm os.FileMode) uint32 {
	v := uint32(perm & 07777)
	if perm&os.ModeSetuid != 0 {
		v |= 04000
	}
	if perm&os.ModeSetgid != 0 {
		v |= 02000
	}
	if perm&os.ModeSticky != 0 {
		v |= 01000
	}
	return v
}

// Perms returns a Step that changes the permissions of a file or directory
// using chmod. Only permission, sticky, setuid, and setgid bits are passed
// to chmod; file type bits (such as os.ModeDir) are masked out.
func Perms(path string, perm os.FileMode) Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "chmod", fmt.Sprintf("%o", chmodBits(perm)), path)
	})
}

// CopyDir returns a Step that copies a directory recursively using cp -r.
func CopyDir(srcDir, dstDir string) Step {
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "cp", "-r", srcDir, dstDir)
	})
}

// RSync returns a Step that synchronizes files and directories using rsync.
func RSync(src, dst string, args ...string) Step {
	allArgs := append([]string{"-a", "--delete"}, args...)
	allArgs = append(allArgs, src, dst)
	return StepFunc(func(ctx context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		return cmdRunner.Run(ctx, "rsync", allArgs...)
	})
}

// WriteFile returns a Step that writes data to the specified path with the specified permissions.
func WriteFile(data []byte, perm os.FileMode, elems ...string) Step {
	return StepFunc(func(_ context.Context, cmdRunner *CommandRunner) (StepResult, error) {
		path := filepath.Join(elems...)
		if cmdRunner.DryRun() {
			return NewStepResult("write "+path, []string{path}, nil, nil), nil
		}
		err := os.WriteFile(path, data, perm)
		return NewStepResult("os.WriteFile", []string{path, fmt.Sprintf("%o", perm)}, nil, err), err
	})
}

// WriteJSONFile returns a Step that marshals v to JSON and writes it to the specified path with the specified permissions.
func WriteJSONFile(v any, elems ...string) Step {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ErrorStep(fmt.Errorf("failed to marshal json: %w", err), "json.Marshal")
	}
	return WriteFile(data, 0644, elems...)
}

// WritePlistFile returns a Step that marshals v to a plist and writes it to the specified path.
func WritePlistFile(v any, elems ...string) Step {
	path := filepath.Join(elems...)
	return writeInfoPlist(path, v)
}
