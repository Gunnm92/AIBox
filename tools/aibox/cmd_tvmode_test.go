package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// e0 : environnement de test avec un $HOME et un XDG_RUNTIME_DIR temporaires,
// et un systemctl simulé (les appels sont enregistrés).
func e0(t *testing.T) (env, *[]string) {
	t.Helper()
	var appels []string
	e := env{home: t.TempDir(), runtime: t.TempDir()}
	e.executer = func(_ []string, nom string, args ...string) int {
		appels = append(appels, nom+" "+strings.Join(args, " "))
		return 0
	}
	return e, &appels
}

func TestTvModeGarde(t *testing.T) {
	e, appels := e0(t)
	var sortie, erreurs bytes.Buffer
	if code := tvMode(e, []string{"gamescope"}, &sortie, &erreurs); code != 0 {
		t.Fatalf("code = %d : %s", code, erreurs.String())
	}
	b, _ := os.ReadFile(filepath.Join(e.home, ".config", "aibox", "tv-mode"))
	if strings.TrimSpace(string(b)) != "gamescope" {
		t.Errorf("fichier gardé = %q", b)
	}
	if !strings.Contains(sortie.String(), "Session TV : gamescope (redémarrage de la session)") {
		t.Errorf("message = %q", sortie.String())
	}
	if len(*appels) != 1 || (*appels)[0] != "sudo systemctl restart aibox-tv" {
		t.Errorf("appels = %v", *appels)
	}
}

func TestTvModeUnePourSession(t *testing.T) {
	e, _ := e0(t)
	var sortie, erreurs bytes.Buffer
	tvMode(e, []string{"--once", "bureau"}, &sortie, &erreurs)
	if _, err := os.Stat(filepath.Join(e.home, ".config", "aibox", "tv-mode")); err == nil {
		t.Error("--once ne doit pas toucher au mode gardé")
	}
	b, _ := os.ReadFile(filepath.Join(e.runtime, "aibox-tv-mode-once"))
	if strings.TrimSpace(string(b)) != "bureau" {
		t.Errorf("mode pour une session = %q", b)
	}
	if !strings.Contains(sortie.String(), "pour cette session") {
		t.Errorf("message = %q", sortie.String())
	}
}

func TestTvModeAffichageDuMode(t *testing.T) {
	e, _ := e0(t)
	var sortie, erreurs bytes.Buffer
	tvMode(e, nil, &sortie, &erreurs)
	if strings.TrimSpace(sortie.String()) != "hyprland" {
		t.Errorf("sans fichier, attendu hyprland, obtenu %q", sortie.String())
	}
	// Le mode pour une session prime sur le mode gardé.
	os.MkdirAll(filepath.Join(e.home, ".config", "aibox"), 0o755)
	os.WriteFile(filepath.Join(e.home, ".config", "aibox", "tv-mode"), []byte("gamescope\n"), 0o644)
	sortie.Reset()
	tvMode(e, nil, &sortie, &erreurs)
	if strings.TrimSpace(sortie.String()) != "gamescope" {
		t.Errorf("mode gardé attendu, obtenu %q", sortie.String())
	}
	os.WriteFile(filepath.Join(e.runtime, "aibox-tv-mode-once"), []byte("bureau\n"), 0o644)
	sortie.Reset()
	tvMode(e, nil, &sortie, &erreurs)
	if strings.TrimSpace(sortie.String()) != "bureau" {
		t.Errorf("mode d'une session attendu, obtenu %q", sortie.String())
	}
}

func TestTvModeInvalide(t *testing.T) {
	for _, mauvais := range [][]string{{"fenetre"}, {"gamescope", "en-trop"}} {
		e, appels := e0(t)
		var sortie, erreurs bytes.Buffer
		if code := tvMode(e, mauvais, &sortie, &erreurs); code != 1 {
			t.Errorf("%v : code = %d, attendu 1", mauvais, code)
		}
		if len(*appels) != 0 {
			t.Errorf("%v : systemctl appelé malgré l'erreur", mauvais)
		}
		if !strings.Contains(erreurs.String(), "Usage : aibox tv-mode") {
			t.Errorf("%v : usage absent", mauvais)
		}
	}
}
