package main

import (
	"bytes"
	"strings"
	"testing"
)

// faussesCommandes : wlr-randr simulé, avec une sortie fixe, et l'appel enregistré.
func faussesCommandes(e *env, sortieWlr string) *[]string {
	var appels []string
	e.lancer = func(vars []string, nom string, args ...string) (string, error) {
		appels = append(appels, nom+" "+strings.Join(args, " "))
		return sortieWlr, nil
	}
	e.executer = func(vars []string, nom string, args ...string) int {
		appels = append(appels, nom+" "+strings.Join(args, " "))
		return 0
	}
	return &appels
}

const wlrSortie = `HEADLESS-1 "Virtual"
  Modes:
    1920x1080 px, 60.000000 Hz (preferred, current)
    1280x720 px, 60.000000 Hz
`

func TestResolutionLecture(t *testing.T) {
	e := env{}
	faussesCommandes(&e, wlrSortie)
	var sortie, erreurs bytes.Buffer
	if code := resolution(e, nil, &sortie, &erreurs); code != 0 {
		t.Fatalf("code = %d, erreurs : %s", code, erreurs.String())
	}
	if got := strings.TrimSpace(sortie.String()); got != "1920x1080" {
		t.Errorf("taille actuelle = %q, attendu 1920x1080", got)
	}
}

func TestResolutionChoisie(t *testing.T) {
	e := env{}
	appels := faussesCommandes(&e, "")
	for _, mode := range []string{"1280x720", "1920x1080@60"} {
		*appels = nil
		var sortie, erreurs bytes.Buffer
		if code := resolution(e, []string{mode}, &sortie, &erreurs); code != 0 {
			t.Fatalf("%s : code = %d", mode, code)
		}
		attendu := "wlr-randr --output HEADLESS-1 --custom-mode " + mode
		if len(*appels) != 1 || (*appels)[0] != attendu {
			t.Errorf("%s : appels = %v, attendu %q", mode, *appels, attendu)
		}
	}
}

func TestResolutionClient(t *testing.T) {
	t.Setenv("SUNSHINE_CLIENT_WIDTH", "2560")
	t.Setenv("SUNSHINE_CLIENT_HEIGHT", "1440")
	t.Setenv("SUNSHINE_CLIENT_FPS", "120")
	e := env{}
	appels := faussesCommandes(&e, "")
	var sortie, erreurs bytes.Buffer
	resolution(e, []string{"client"}, &sortie, &erreurs)
	if len(*appels) != 1 || !strings.HasSuffix((*appels)[0], "--custom-mode 2560x1440@120") {
		t.Errorf("appels = %v", *appels)
	}
}

func TestResolutionArgumentInvalide(t *testing.T) {
	e := env{}
	faussesCommandes(&e, "")
	for _, mauvais := range []string{"x", "1920", "grand", "1920x1080@"} {
		var sortie, erreurs bytes.Buffer
		if code := resolution(e, []string{mauvais}, &sortie, &erreurs); code != 1 {
			t.Errorf("%q : code = %d, attendu 1", mauvais, code)
		}
		if !strings.Contains(erreurs.String(), "Usage : aibox resolution") {
			t.Errorf("%q : pas d'usage affiché", mauvais)
		}
	}
}
