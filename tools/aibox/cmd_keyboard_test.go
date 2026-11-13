package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clavierTest : racine avec la liste xkb (layouts fr, ch ; variante latin9) et
// une disposition locale « aibox » (variante fr_mac).
func clavierTest(t *testing.T) (env, *[]string) {
	t.Helper()
	racine := t.TempDir()
	ecrire := func(rel, contenu string) {
		p := filepath.Join(racine, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(contenu), 0o644)
	}
	ecrire("usr/share/X11/xkb/rules/base.lst", `! model
  pc105          Generic 105-key PC

! layout
  us             English (US)
  fr             French
  ch             Swiss

! variant
  latin9         fr: French (alternative, latin9)
  nodeadkeys     ch: German (no dead keys)

`)
	ecrire("etc/xkb/symbols/aibox", `default partial alphanumeric_keys
xkb_symbols "basic" {
};
xkb_symbols "fr_mac" {
};
`)
	ecrire("etc/aibox/keyboard.env", "KEYBOARD_LAYOUT=aibox\nKEYBOARD_VARIANT=fr_mac\n")

	var appels []string
	e := env{racine: racine, home: t.TempDir()}
	e.executer = func(_ []string, nom string, args ...string) int {
		appels = append(appels, nom+" "+strings.Join(args, " "))
		return 0
	}
	return e, &appels
}

func TestKeyboardAffichage(t *testing.T) {
	e, _ := clavierTest(t)
	var sortie, erreurs bytes.Buffer
	keyboard(e, nil, &sortie, &erreurs)
	if strings.TrimSpace(sortie.String()) != "Clavier : aibox (fr_mac)" {
		t.Errorf("affichage = %q", sortie.String())
	}
}

func TestKeyboardMacEtPc(t *testing.T) {
	e, appels := clavierTest(t)
	var sortie, erreurs bytes.Buffer
	if code := keyboard(e, []string{"pc"}, &sortie, &erreurs); code != 0 {
		t.Fatalf("pc : code = %d : %s", code, erreurs.String())
	}
	b, _ := os.ReadFile(filepath.Join(e.home, ".config", "aibox", "keyboard.env"))
	if string(b) != "KEYBOARD_LAYOUT=fr\nKEYBOARD_VARIANT=\n" {
		t.Errorf("fichier = %q", b)
	}
	if len(*appels) != 1 || (*appels)[0] != "systemctl --user restart aibox-admin-wayvnc.service" {
		t.Errorf("appels = %v", *appels)
	}
	keyboard(e, []string{"mac"}, &sortie, &erreurs)
	b, _ = os.ReadFile(filepath.Join(e.home, ".config", "aibox", "keyboard.env"))
	if string(b) != "KEYBOARD_LAYOUT=aibox\nKEYBOARD_VARIANT=fr_mac\n" {
		t.Errorf("mac : fichier = %q", b)
	}
}

func TestKeyboardDispositionEtVarianteInconnues(t *testing.T) {
	e, appels := clavierTest(t)
	var sortie, erreurs bytes.Buffer
	if code := keyboard(e, []string{"xx"}, &sortie, &erreurs); code != 1 || !strings.Contains(erreurs.String(), "Disposition inconnue : xx") {
		t.Errorf("disposition : code %d, %q", code, erreurs.String())
	}
	erreurs.Reset()
	if code := keyboard(e, []string{"fr", "inconnue"}, &sortie, &erreurs); code != 1 {
		t.Errorf("variante : code = %d", code)
	}
	if !strings.Contains(erreurs.String(), "Variantes disponibles") || !strings.Contains(erreurs.String(), "latin9") {
		t.Errorf("liste des variantes absente : %q", erreurs.String())
	}
	if len(*appels) != 0 {
		t.Error("wayvnc ne doit pas être relancé pour une disposition refusée")
	}
	if _, err := os.Stat(filepath.Join(e.home, ".config", "aibox", "keyboard.env")); err == nil {
		t.Error("le réglage ne doit pas être écrit pour une disposition refusée")
	}
}

func TestKeyboardVarianteValide(t *testing.T) {
	e, _ := clavierTest(t)
	var sortie, erreurs bytes.Buffer
	if code := keyboard(e, []string{"ch", "nodeadkeys"}, &sortie, &erreurs); code != 0 {
		t.Errorf("code = %d : %s", code, erreurs.String())
	}
}
