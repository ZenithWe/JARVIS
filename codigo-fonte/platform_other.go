//go:build !windows

package main

import "errors"

func protect(s string) (string, error) {
	return "", errors.New("Persistência de credenciais disponível apenas no Windows (DPAPI)")
}
func unprotect(s string) (string, error) { return "", errors.New("Credencial Windows") }
func openURL(s string) error             { return errors.New("Abertura de aplicativos disponível no Windows") }
func openUI(s string) error              { return openURL(s) }
func launchApp(s string) error           { return openURL(s) }