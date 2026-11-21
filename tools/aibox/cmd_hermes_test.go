package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestCommandeHermes : sous sudo, hermes est pris dans le home de l'utilisateur
// d'origine et lancé avec runuser ; sans sudo, dans le PATH ; absent, rien.
func TestCommandeHermes(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	chemin := filepath.Join(bin, "hermes")
	if err := os.WriteFile(chemin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	nom, prefixe, ok := commandeHermes(home, "alice", 0)
	if !ok || nom != "runuser" || !reflect.DeepEqual(prefixe, []string{"-u", "alice", "--", chemin}) {
		t.Errorf("sudo : %q %v %v, attendu runuser -u alice -- %s", nom, prefixe, ok, chemin)
	}

	if _, _, ok := commandeHermes(t.TempDir(), "alice", 0); ok {
		t.Error("sudo sans hermes dans le home : attendu absent")
	}
}
