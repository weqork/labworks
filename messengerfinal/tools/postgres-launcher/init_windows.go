package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func randomPassword() (string, error) {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func shortPath(path string) string {
	input, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return path
	}
	buffer := make([]uint16, 32768)
	api := syscall.NewLazyDLL("kernel32.dll").NewProc("GetShortPathNameW")
	size, _, _ := api.Call(uintptr(unsafe.Pointer(input)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if size == 0 || size >= uintptr(len(buffer)) {
		return path
	}
	return syscall.UTF16ToString(buffer[:size])
}

func initialize(root string) error {
	lockPath := filepath.Join(root, ".postgres-init.lock")
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("не удалось начать подготовку базы: %w", err)
	}
	lock.Close()
	defer os.Remove(lockPath)

	data := filepath.Join(root, "data")
	if _, err := os.Stat(data); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	config := filepath.Join(root, ".env")
	if _, err := os.Stat(config); err == nil {
		return fmt.Errorf("найден %s, но нет data. Для новой базы переместите старый .env в резервную папку", config)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	stage, err := os.MkdirTemp(root, ".postgres-init-")
	if err != nil {
		return err
	}
	adminPassword, err := randomPassword()
	if err != nil {
		return err
	}
	appPassword, err := randomPassword()
	if err != nil {
		return err
	}
	adminFile := filepath.Join(stage, "admin-password.txt")
	if err := os.WriteFile(adminFile, []byte(adminPassword+"\n"), 0600); err != nil {
		return err
	}
	defer os.Remove(adminFile)
	stageData := filepath.Join(shortPath(stage), "data")
	pgDir := filepath.Join(shortPath(root), "pgsql")
	binDir := filepath.Join(pgDir, "bin")
	log := filepath.Join(root, "postgres.log")
	out, err := command(filepath.Join(binDir, "initdb.exe"),
		"-D", stageData, "-L", filepath.Join(pgDir, "share"),
		"-U", "postgres", "-A", "scram-sha-256", "--pwfile="+shortPath(adminFile),
		"--encoding=UTF8", "--locale=C",
		"-c", "listen_addresses=127.0.0.1", "-c", "port=55432")
	if err != nil {
		return fmt.Errorf("не удалось создать каталог данных: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	if err := start(stageData, binDir, log); err != nil {
		return err
	}

	sql := "CREATE ROLE messenger_app LOGIN PASSWORD '" + appPassword + "';\n" +
		"CREATE DATABASE messenger OWNER messenger_app;\n"
	psql := exec.Command(filepath.Join(binDir, "psql.exe"), "-X", "-h", "127.0.0.1", "-p", "55432",
		"-U", "postgres", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-q")
	psql.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	psql.Stdin = strings.NewReader(sql)
	psql.Env = append(os.Environ(), "PGPASSWORD="+adminPassword, "PGSSLMODE=disable", "PGCONNECT_TIMEOUT=5")
	_, setupErr := psql.CombinedOutput()
	stopOut, stopErr := command(filepath.Join(binDir, "pg_ctl.exe"), "-D", stageData, "-m", "fast", "-w", "stop")
	if stopErr != nil {
		return fmt.Errorf("не удалось остановить базу после настройки: %w\n%s", stopErr, strings.TrimSpace(string(stopOut)))
	}
	if setupErr != nil {
		return fmt.Errorf("не удалось создать базу messenger: %w", setupErr)
	}

	content := "PGHOST=127.0.0.1\r\nPGPORT=55432\r\nPGDATABASE=messenger\r\nPGUSER=messenger_app\r\nPGPASSWORD=" + appPassword + "\r\n"
	if err := os.WriteFile(config, []byte(content), 0600); err != nil {
		return err
	}
	if err := os.Rename(stageData, data); err != nil {
		os.Remove(config)
		return err
	}
	os.Remove(adminFile)
	os.Remove(stage)
	return nil
}
