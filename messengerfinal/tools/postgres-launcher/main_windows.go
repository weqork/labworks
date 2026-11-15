package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func command(path string, args ...string) ([]byte, error) {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.CombinedOutput()
}

func start(dataDir, binDir, logPath string) error {
	version, err := os.ReadFile(filepath.Join(dataDir, "PG_VERSION"))
	if err != nil {
		return fmt.Errorf("Не удалось найти существующую базу в %s: %w", dataDir, err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "global", "pg_control")); err != nil {
		return fmt.Errorf("Не найден файл управления существующей базой: %w", err)
	}
	postgres := filepath.Join(binDir, "postgres.exe")
	output, err := command(postgres, "--version")
	if err != nil {
		return fmt.Errorf("Не удалось запустить %s: %w", postgres, err)
	}
	parts := strings.Fields(string(output))
	if len(parts) < 3 || strings.SplitN(parts[2], ".", 2)[0] != strings.TrimSpace(string(version)) {
		return fmt.Errorf("Версия PostgreSQL не совпадает с версией каталога данных: %s", strings.TrimSpace(string(output)))
	}

	pgctl := filepath.Join(binDir, "pg_ctl.exe")
	output, err = command(pgctl, "-D", dataDir, "status")
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		return fmt.Errorf("Не удалось проверить статус PostgreSQL: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	cmd := exec.Command(pgctl, "-D", dataDir, "-l", logPath, "-w", "-t", "30", "start")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("PostgreSQL не запустился: %w\n\nЖурнал: %s", err, logPath)
	}
	return nil
}

func showMessage(message string, icon uintptr) {
	box := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	title, _ := syscall.UTF16PtrFromString("PostgreSQL — Messenger")
	body, _ := syscall.UTF16PtrFromString(message)
	box.Call(0, uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), icon)
}

func main() {
	quiet := len(os.Args) == 2 && os.Args[1] == "--quiet"
	executable, err := os.Executable()
	root := filepath.Dir(executable)
	dataDir := filepath.Join(root, "data")
	logPath := filepath.Join(root, "postgres.log")
	if err == nil {
		if _, statErr := os.Stat(dataDir); errors.Is(statErr, os.ErrNotExist) {
			err = initialize(root)
		}
	}
	if err == nil {
		err = start(dataDir, filepath.Join(root, "pgsql", "bin"), logPath)
	}
	if err != nil {
		if !quiet {
			showMessage(err.Error(), 0x10)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
	if !quiet {
		showMessage("PostgreSQL работает в фоне на 127.0.0.1:55432.\n\nБаза: messenger\nДанные: "+dataDir, 0x40)
	}
}
