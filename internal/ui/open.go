package ui

import (
	"os/exec"
	"runtime"
)

// openerArgv returns the command that hands a target to the desktop's
// default handler on the given OS.
//
// Windows goes through rundll32 rather than "cmd /c start": start treats &
// specially and reads a leading quoted argument as a window title, and
// cmd.exe re-parses the command line with rules that differ from the ones
// Go escapes for. A link target comes from a document, so it must not reach
// a shell.
func openerArgv(goos, target string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{target}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", target}
	default:
		return "xdg-open", []string{target}
	}
}

// systemOpen launches the target in the desktop's default handler.
func systemOpen(target string) error {
	name, args := openerArgv(runtime.GOOS, target)
	return exec.Command(name, args...).Start()
}
