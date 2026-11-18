package hostopen

import "path"

// FileManagerArgv shows p in the OS file manager: a folder is opened, a file is selected in
// its folder (Linux has no standard "select", so its folder is opened).
func FileManagerArgv(goos, p string, isDir bool) []string {
	switch goos {
	case "windows":
		if isDir {
			return []string{"explorer.exe", p}
		}
		return []string{"explorer.exe", "/select," + p}
	case "darwin":
		if isDir {
			return []string{"open", p}
		}
		return []string{"open", "-R", p}
	}
	if isDir {
		return []string{"xdg-open", p}
	}
	return []string{"xdg-open", path.Dir(p)}
}
