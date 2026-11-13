package main

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// remplacementFaux enregistre l'appel au lieu de remplacer le processus.
func remplacementFaux(appel *[]string, vars *[]string) func(string, []string, []string) error {
	return func(chemin string, argv []string, v []string) error {
		*appel = append([]string{chemin}, argv...)
		*vars = v
		return nil
	}
}

func TestTvRunSansSocket(t *testing.T) {
	e := env{runtime: t.TempDir()}
	var appel, vars []string
	var erreurs bytes.Buffer
	if code := tvRun(e, []string{"true"}, &erreurs, remplacementFaux(&appel, &vars)); code != 1 {
		t.Errorf("code = %d, attendu 1", code)
	}
	if appel != nil {
		t.Error("la commande ne doit pas être lancée sans la TV")
	}
	if !strings.Contains(erreurs.String(), "aibox tv-mode --once hyprland") {
		t.Errorf("message = %q", erreurs.String())
	}
}

func TestTvRunAvecSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "tv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	ecoute, err := net.Listen("unix", filepath.Join(dir, "wayland-1"))
	if err != nil {
		t.Skipf("socket Unix impossible ici : %v", err)
	}
	defer ecoute.Close()

	e := env{runtime: dir}
	var appel, vars []string
	var erreurs bytes.Buffer
	if code := tvRun(e, []string{"env", "WSQUASHFS_HIDRAW=1", "jeu"}, &erreurs, remplacementFaux(&appel, &vars)); code != 0 {
		t.Fatalf("code = %d : %s", code, erreurs.String())
	}
	if len(appel) < 2 || !strings.HasSuffix(appel[0], "/env") || appel[1] != "env" {
		t.Errorf("appel = %v", appel)
	}
	if strings.Join(vars, " ") != "WAYLAND_DISPLAY=wayland-1 DISPLAY=:1" {
		t.Errorf("variables = %v", vars)
	}
}

func TestTvRunSansArgument(t *testing.T) {
	dir, err := os.MkdirTemp("", "tv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	ecoute, err := net.Listen("unix", filepath.Join(dir, "wayland-1"))
	if err != nil {
		t.Skipf("socket Unix impossible ici : %v", err)
	}
	defer ecoute.Close()

	e := env{runtime: dir}
	var appel, vars []string
	var erreurs bytes.Buffer
	if code := tvRun(e, nil, &erreurs, remplacementFaux(&appel, &vars)); code != 2 {
		t.Errorf("code = %d, attendu 2", code)
	}
	if !strings.Contains(erreurs.String(), "Usage : aibox tv-run") {
		t.Errorf("usage absent : %q", erreurs.String())
	}
}
