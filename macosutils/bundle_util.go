// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

// Package macosutils contains macos specific utilities.
package macosutils

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// IsAppBundle returns true if path is a directory ending with .app and contains
// a Contents/Info.plist file.
func IsAppBundle(path string) bool {
	path = filepath.Clean(path)
	if !strings.HasSuffix(strings.ToLower(path), ".app") {
		return false
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}

	plistPath := filepath.Join(path, "Contents", "Info.plist")
	plistInfo, err := os.Stat(plistPath)
	if err != nil || plistInfo.IsDir() {
		return false
	}

	return true
}

// IsExecutable returns true if the provided file mode has any of the executable
// bits set (ie  mode&0o111 != 0).
func IsExecutable(mode fs.FileMode) bool {
	return mode&0o111 != 0
}

// IsReadable returns true if the provided file mode has any of the readable
// bits set (ie  mode&0o444 != 0).
func IsReadable(mode fs.FileMode) bool {
	return mode&0o444 != 0
}

// LocateInBundle finds the requested file whose permissions are matched by
// the matchPerms function, eg. use IsExecutable to find any file with an executable
// bit set. It will descend into subpackages to locate the requested file.
// If matchPerms is nil, IsExecutable is used. The returned path is absolute.
func LocateInBundle(bundlePath, filename string, matchPerms func(fs.FileMode) bool) (string, bool) {
	if !IsAppBundle(bundlePath) {
		return "", false
	}
	if matchPerms == nil {
		matchPerms = IsExecutable
	}

	// Confine the walk to the bundle directory: os.Root rejects any path that
	// leaves it, including via symlinks inside the bundle. This matters because
	// the binary located here is subsequently executed.
	root, err := os.OpenRoot(bundlePath)
	if err != nil {
		return "", false
	}
	defer root.Close()

	location := ""
	_ = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == filename {
			if info, err := d.Info(); err == nil && info.Mode().IsRegular() && matchPerms(info.Mode().Perm()) {
				location = filepath.Join(bundlePath, path)
				return fs.SkipAll
			}
		}
		return nil
	})
	return location, location != ""
}

// LocateInNestedBundle finds the requested file in its immediately enclosing
// bundle specified by bundle, and if not found, then in the bundle enclosing
// that one, and so on, until a match is found or a non-bundle directory is reached.
// The returned path is absolute. The parents are the expected names of the enclosing
// directories, starting with the top-level directory directly inside the bundle
// and ending with the immediate parent of bundle (i.e. top-down order, matching InBundle).
// If any of the parents do not match, or if the top-level parent is not a bundle,
// then no match is found. The search stops at the first match, so if a file exists
// in multiple bundles, only the innermost one is returned.
//
// This function is useful when a file may be located in a bundle nested inside
// another bundle, and you want to find it starting from the inner bundle and
// searching outwards. For example, if you have an app bundle that contains a
// nested framework bundle, and you want to locate a resource file that may be
// in either the framework or the app bundle, you can use this function to search
// for it starting from the framework bundle.
func LocateInNestedBundle(bundle, filename string, matchPerms func(fs.FileMode) bool, parents ...string) (string, bool) {
	for {
		var ok bool
		bundle, ok = InBundle(bundle, parents...)
		if !ok {
			return "", false
		}
		if fp, ok := LocateInBundle(bundle, filename, matchPerms); ok {
			return fp, true
		}
		// Continue from the bundle itself: InBundle resolves a path to the
		// bundle enclosing it, so the bundle just searched is the next path to
		// resolve. Taking its Dir instead would step to the enclosing
		// Contents/MacOS, which is not itself in a Contents/MacOS.
	}
}

// ExecutablePath returns the path of the executable that started the
// current process, following softlinks.
//
// Caveat for sandboxed processes started with a relative path: this is
// os.Executable(), which, on Darwin, returns the path exactly as passed to
// execve, resolving it against the current directory only if it was
// relative. The App Sandbox changes the process's current directory to its
// container (e.g. ~/Library/Containers/<bundle-id>/Data) as part of exec,
// before any Go code runs, so by the time ExecutablePath resolves a relative
// path, it joins it with the container directory, not the directory the
// process was actually launched from. The result is a path that does not
// exist. There is no way to recover the original directory once this has
// happened.
//
// This only affects a sandboxed binary launched with a relative path, for
// example by running "build/Foo.app/Contents/MacOS/Foo" directly from a
// shell to see its output live rather than via Console.app. It does not
// affect `open`, even given a relative path, since `open` resolves the path
// to absolute itself before launching the app. Launching with an absolute
// path avoids the problem entirely.
func ExecutablePath() (string, error) {
	e, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(e); err == nil {
		e = resolved
	}
	return e, nil
}

// InBundle returns true if the specified path has the specified parents
// and the top-level parent is an app bundle, that is ends in .app
// and contains a Contents/Info.plist file.
func InBundle(path string, parents ...string) (string, bool) {
	if len(parents) == 0 {
		return "", false
	}
	parent := filepath.Dir(path)
	for _, parent0 := range slices.Backward(parents) {
		if !strings.EqualFold(filepath.Base(parent), parent0) {
			return "", false
		}
		parent = filepath.Dir(parent)
	}
	// The loop has consumed every parent, so parent is now the candidate
	// bundle itself; taking its Dir again would step outside it.
	appBundle := parent
	if !IsAppBundle(appBundle) {
		return "", false
	}
	return appBundle, true
}

// ProcessInBundle determines if the executable that started the running process
// is within an app bundle and returns the path of that bundle. It uses
// InBundle(executable, "Contents", "MacOS") as the heuristic; use
// LocateInBundle to then find a file within the returned bundle.
//
// A bundle nested inside another must therefore be placed in the outer
// bundle's Contents/MacOS, ie. <outer>.app/Contents/MacOS/<inner>.app, so that
// the same heuristic resolves the inner bundle to the outer one. A bundle
// placed in Contents/Library is reachable by LocateInBundle, which walks the
// whole tree, but not by InBundle or ProcessInBundle.
//
// For a sandboxed process, this inherits the caveat documented on
// ExecutablePath: if the process was launched with a relative path directly
// (not via `open`), the executable path, and so the bundle path, will be
// wrong, and ProcessInBundle will report false negatives (or, in principle,
// match an unrelated bundle that happens to exist at the bogus path).
func ProcessInBundle() (string, bool) {
	exe, err := ExecutablePath()
	if err != nil {
		return "", false
	}
	return InBundle(exe, "Contents", "MacOS")
}

// LookPathBundle is like exec.LookPath but for app bundles.
func LookPathBundle(bundle, pathList string) string {
	found := LookPathBundleAll(bundle, pathList)
	if len(found) > 0 {
		return found[0]
	}
	return ""
}

// LookPathBundleAll is like LookPathBundle but returns all instances
// of bundle on pathList without duplicates.
func LookPathBundleAll(bundle, pathList string) []string {
	if filepath.IsAbs(bundle) {
		if IsAppBundle(bundle) {
			return []string{bundle}
		}
		return nil
	}

	if bundle != filepath.Base(bundle) || bundle == "." || bundle == ".." {
		return nil
	}

	var found []string
	dirs := filepath.SplitList(pathList)
	seen := make(map[string]struct{}, len(dirs))
	for _, dir := range dirs {
		dir = filepath.Clean(dir)
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		candidate := filepath.Join(dir, bundle)
		if IsAppBundle(candidate) {
			found = append(found, candidate)
		}
	}
	return found
}

// LookupBundleBinary iterates over all instances of bundle in pathList to locate
// the first one that contains binary returning the absolute pathname of the bundle
// and binary in that bundle or empty strings if not found.
func LookupBundleBinary(bundle, binary, pathList string) (string, string, bool) {
	bundlePaths := LookPathBundleAll(bundle, pathList)
	for _, bundle := range bundlePaths {
		if bp, ok := LocateInBundle(bundle, binary, IsExecutable); ok {
			return bundle, bp, true
		}
	}
	return "", "", false
}
