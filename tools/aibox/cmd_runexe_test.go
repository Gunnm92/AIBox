package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// environnementRunExe : racine, $HOME et Steam temporaires, avec un Proton cachyos.
func environnementRunExe(t *testing.T) env {
	t.Helper()
	e := env{racine: t.TempDir(), home: t.TempDir()}
	for _, v := range []string{"proton-cachyos-9.0", "proton-cachyos-10.0"} {
		os.MkdirAll(filepath.Join(e.home, ".local", "share", "Steam", "compatibilitytools.d", v), 0o755)
	}
	return e
}

// remplaceurFaux enregistre argv, variables et dossier au lieu de lancer umu-run.
func remplaceurFaux(argv, vars *[]string, dossier *string) remplaceurUmu {
	return func(a, v []string, d, _ string) error {
		*argv = a
		*vars = v
		*dossier = d
		return nil
	}
}

func TestRunExeSansArgument(t *testing.T) {
	var argv, vars []string
	var dossier string
	var erreurs bytes.Buffer
	if code := runExe(environnementRunExe(t), nil, &erreurs, remplaceurFaux(&argv, &vars, &dossier)); code != 1 {
		t.Errorf("code = %d, attendu 1", code)
	}
	if !strings.Contains(erreurs.String(), "Usage : aibox run-exe") {
		t.Errorf("usage absent")
	}
}

func TestRunExeFichierAbsent(t *testing.T) {
	var argv, vars []string
	var dossier string
	var erreurs bytes.Buffer
	if code := runExe(environnementRunExe(t), []string{"absent.exe"}, &erreurs, remplaceurFaux(&argv, &vars, &dossier)); code != 1 {
		t.Errorf("code = %d, attendu 1", code)
	}
	if argv != nil {
		t.Error("umu-run ne doit pas être lancé sans programme")
	}
}

func TestRunExeChoixDuProtonLePlusRecent(t *testing.T) {
	e := environnementRunExe(t)
	prog := filepath.Join(t.TempDir(), "jeu.exe")
	os.WriteFile(prog, []byte("MZ"), 0o644)
	var argv, vars []string
	var dossier string
	var erreurs bytes.Buffer
	if code := runExe(e, []string{prog, "-x"}, &erreurs, remplaceurFaux(&argv, &vars, &dossier)); code != 0 {
		t.Fatalf("code = %d : %s", code, erreurs.String())
	}
	if !strings.Contains(strings.Join(vars, " "), "PROTONPATH="+filepath.Join(e.home, ".local/share/Steam/compatibilitytools.d", "proton-cachyos-10.0")) {
		t.Errorf("mauvais Proton : %v", vars)
	}
	if strings.Join(argv, " ") != "umu-run "+prog+" -x" {
		t.Errorf("argv = %v", argv)
	}
	if dossier != filepath.Dir(prog) {
		t.Errorf("dossier = %q", dossier)
	}
}

func TestRunExeProtonDeProtonConf(t *testing.T) {
	e := environnementRunExe(t)
	os.MkdirAll(filepath.Join(e.home, ".config", "aibox"), 0o755)
	os.WriteFile(filepath.Join(e.home, ".config", "aibox", "proton.conf"), []byte("PROTON=\"proton-cachyos-9.0\"\n"), 0o644)
	prog := filepath.Join(t.TempDir(), "jeu.exe")
	os.WriteFile(prog, []byte("MZ"), 0o644)
	var argv, vars []string
	var dossier string
	var erreurs bytes.Buffer
	runExe(e, []string{prog}, &erreurs, remplaceurFaux(&argv, &vars, &dossier))
	if !strings.Contains(strings.Join(vars, " "), "proton-cachyos-9.0") {
		t.Errorf("proton.conf ignoré : %v", vars)
	}
}

func TestRunExeMsi(t *testing.T) {
	e := environnementRunExe(t)
	prog := filepath.Join(t.TempDir(), "setup.MSI")
	os.WriteFile(prog, []byte("x"), 0o644)
	var argv, vars []string
	var dossier string
	var erreurs bytes.Buffer
	runExe(e, []string{prog, "/quiet"}, &erreurs, remplaceurFaux(&argv, &vars, &dossier))
	if strings.Join(argv, " ") != "umu-run msiexec /i "+prog+" /quiet" {
		t.Errorf("argv = %v", argv)
	}
}

func TestRunExeJournal(t *testing.T) {
	e := environnementRunExe(t)
	prog := filepath.Join(t.TempDir(), "jeu.exe")
	os.WriteFile(prog, []byte("MZ"), 0o644)
	var argv, vars []string
	var dossier string
	var erreurs bytes.Buffer
	runExe(e, []string{prog}, &erreurs, remplaceurFaux(&argv, &vars, &dossier))
	b, err := os.ReadFile(filepath.Join(e.home, ".cache", "aibox", "run-exe", "jeu.exe.log"))
	if err != nil {
		t.Fatalf("journal absent : %v", err)
	}
	if !strings.Contains(string(b), "Préfixe : ") || !strings.Contains(string(b), "jeu.exe") {
		t.Errorf("entête du journal incomplet : %q", b)
	}
}

func TestVersionAvant(t *testing.T) {
	cas := []struct {
		a, b string
		vrai bool
	}{
		{"proton-9.0", "proton-10.0", true},
		{"proton-10.0", "proton-9.0", false},
		{"proton-cachyos-10-2", "proton-cachyos-10-10", true},
		{"a", "a", false},
	}
	for _, c := range cas {
		if got := versionAvant(c.a, c.b); got != c.vrai {
			t.Errorf("versionAvant(%q, %q) = %v, attendu %v", c.a, c.b, got, c.vrai)
		}
	}
}
