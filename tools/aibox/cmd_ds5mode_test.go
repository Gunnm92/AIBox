package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDs5ModeParDefaut(t *testing.T) {
	e, appels := e0(t)
	var sortie, erreurs bytes.Buffer
	if code := ds5Mode(e, nil, &sortie, &erreurs); code != 0 {
		t.Fatalf("code = %d : %s", code, erreurs.String())
	}
	if strings.TrimSpace(sortie.String()) != "natif" {
		t.Errorf("sans fichier, attendu natif, obtenu %q", sortie.String())
	}
	if len(*appels) != 0 {
		t.Errorf("affichage ne doit lancer aucune commande : %v", *appels)
	}
}

func TestDs5ModeLitLeFichier(t *testing.T) {
	e, _ := e0(t)
	os.MkdirAll(filepath.Join(e.home, ".config", "aibox"), 0o755)
	os.WriteFile(filepath.Join(e.home, ".config", "aibox", "ds5-mode"), []byte("son\n"), 0o644)
	var sortie, erreurs bytes.Buffer
	ds5Mode(e, nil, &sortie, &erreurs)
	if strings.TrimSpace(sortie.String()) != "son" {
		t.Errorf("attendu son, obtenu %q", sortie.String())
	}
}

func TestDs5ModeEcritEtLanceSystemd(t *testing.T) {
	cas := []struct {
		mode     string
		commande string
		regle    string
	}{
		{"natif", "systemctl --user stop ds5-haptics-loopback", "Off"},
		{"son", "systemctl --user start ds5-haptics-loopback", "Replace"},
	}
	for _, c := range cas {
		e, appels := e0(t)
		var sortie, erreurs bytes.Buffer
		if code := ds5Mode(e, []string{c.mode}, &sortie, &erreurs); code != 0 {
			t.Fatalf("%s : code = %d : %s", c.mode, code, erreurs.String())
		}
		b, _ := os.ReadFile(filepath.Join(e.home, ".config", "aibox", "ds5-mode"))
		if strings.TrimSpace(string(b)) != c.mode {
			t.Errorf("%s : fichier = %q", c.mode, b)
		}
		if len(*appels) != 1 || (*appels)[0] != c.commande {
			t.Errorf("%s : appels = %v, attendu %q", c.mode, *appels, c.commande)
		}
		if !strings.Contains(sortie.String(), "« "+c.regle+" »") {
			t.Errorf("%s : réglage du dongle absent du message : %q", c.mode, sortie.String())
		}
	}
}

func TestDs5ModeInvalide(t *testing.T) {
	for _, mauvais := range [][]string{{"midi"}, {"son", "en-trop"}} {
		e, appels := e0(t)
		var sortie, erreurs bytes.Buffer
		if code := ds5Mode(e, mauvais, &sortie, &erreurs); code != 1 {
			t.Errorf("%v : code = %d, attendu 1", mauvais, code)
		}
		if len(*appels) != 0 {
			t.Errorf("%v : systemctl appelé malgré l'erreur", mauvais)
		}
		if !strings.Contains(erreurs.String(), "Usage : aibox ds5-mode") {
			t.Errorf("%v : usage absent", mauvais)
		}
		if _, err := os.Stat(filepath.Join(e.home, ".config", "aibox", "ds5-mode")); err == nil {
			t.Errorf("%v : fichier écrit malgré l'erreur", mauvais)
		}
	}
}
