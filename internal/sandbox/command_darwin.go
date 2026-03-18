//go:build darwin

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func seatbeltProfile(home string) string {
	return fmt.Sprintf(`(version 1)
(deny default)
(allow process-exec)
(allow process-fork)
(allow sysctl-read)
(allow mach-lookup)
(allow signal)
(allow ipc-posix*)

;; read-only filesystem
(allow file-read*)

;; writable only under $HOME
(allow file-write*
    (subpath %q))

;; allow network
(allow network*)
`, home)
}

func SandboxCommand(ctx context.Context, lang string) (*exec.Cmd, error) {
	runtime := runtimeMap[lang]
	ext := extMap[lang]

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	wrapperPath := filepath.Join(wd, "internal", "resource", fmt.Sprintf("wrapper%s", ext))

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	profile := seatbeltProfile(homeDir)

	args := []string{"-p", profile, runtime}
	if lang == "python" {
		args = append(args, "-u")
	}
	args = append(args, wrapperPath)

	return exec.CommandContext(ctx, "sandbox-exec", args...), nil
}
