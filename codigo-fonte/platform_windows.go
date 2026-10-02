//go:build windows

package main

import (
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

type dataBlob struct {
	Size uint32
	Data *byte
}

func blob(b []byte) dataBlob {
	v := dataBlob{Size: uint32(len(b))}
	if len(b) > 0 {
		v.Data = &b[0]
	}
	return v
}
func crypt(b []byte, decrypt bool) ([]byte, error) {
	dll := syscall.NewLazyDLL("crypt32.dll")
	name := "CryptProtectData"
	if decrypt {
		name = "CryptUnprotectData"
	}
	in := blob(b)
	var out dataBlob
	r, _, e := dll.NewProc(name).Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(b)
	if r == 0 {
		return nil, e
	}
	defer syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree").Call(uintptr(unsafe.Pointer(out.Data)))
	return append([]byte{}, unsafe.Slice(out.Data, out.Size)...), nil
}
func protect(s string) (string, error) {
	b, e := crypt([]byte(s), false)
	return base64.StdEncoding.EncodeToString(b), e
}
func unprotect(s string) (string, error) {
	b, e := base64.StdEncoding.DecodeString(s)
	if e != nil {
		return "", e
	}
	b, e = crypt(b, true)
	return string(b), e
}
func openURL(u string) error {
	p, e := syscall.UTF16PtrFromString(u)
	if e != nil {
		return e
	}
	verb, _ := syscall.UTF16PtrFromString("open")
	r, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(p)), 0, 0, 1)
	if r <= 32 {
		return errors.New("o Windows não conseguiu abrir o endereço")
	}
	return nil
}
func launchApp(name string) error {
	apps := map[string]string{"calculator": "calc.exe", "notepad": "notepad.exe", "explorer": "explorer.exe"}
	f, ok := apps[name]
	if !ok {
		return errors.New("aplicativo não autorizado")
	}
	root := os.Getenv("WINDIR")
	p := filepath.Join(root, "System32", f)
	if name == "explorer" {
		p = filepath.Join(root, f)
	}
	cmd := exec.Command(p)
	if e := cmd.Start(); e != nil {
		return e
	}
	go cmd.Wait()
	return nil
}
func openUI(u string) error {
	for _, root := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles")} {
		p := filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe")
		if _, e := os.Stat(p); e == nil {
			cmd := exec.Command(p, "--app="+u, "--window-size=1500,960")
			if e = cmd.Start(); e == nil {
				go cmd.Wait()
				return nil
			}
		}
	}
	return openURL(u)
}