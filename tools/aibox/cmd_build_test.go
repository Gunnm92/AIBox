package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// buildTest : système temporaire monté (disque de données), dépôt déjà cloné,
// clés présentes ; git, build.sh et cp sont simulés.
type buildTest struct {
	e        env
	base     string
	appelsFlux []string
	code     int
	version  string
}

func nouveauBuild(t *testing.T) *buildTest {
	t.Helper()
	racine := t.TempDir()
	os.MkdirAll(filepath.Join(racine, "proc", "self"), 0o755)
	os.WriteFile(filepath.Join(racine, "proc", "self", "mountinfo"), []byte(montageDonnees), 0o644)
	bt := &buildTest{version: "2026.11.13.2100-abc1234", base: filepath.Join(racine, "var", "lib", "aibox", "state", "build")}
	os.MkdirAll(filepath.Join(bt.base, "src", ".git"), 0o755)
	os.MkdirAll(filepath.Join(bt.base, "keys"), 0o700)
	os.WriteFile(filepath.Join(bt.base, "keys", "authorized_keys"), []byte("ssh-ed25519 AAAA test\n"), 0o600)

	bt.e = env{racine: racine, home: t.TempDir()}
	bt.e.lancer = func(_ []string, nom string, args ...string) (string, error) {
		if nom == "git" && strings.Contains(strings.Join(args, " "), "--short=7") {
			return "abc1234\n", nil
		}
		if nom == "du" {
			return "1,2G\t" + args[len(args)-1] + "\n", nil
		}
		return "", nil
	}
	bt.e.flux = func(_ []string, sortie io.Writer, nom string, args ...string) int {
		bt.appelsFlux = append(bt.appelsFlux, nom+" "+strings.Join(args, " "))
		if bt.code != 0 {
			sortie.Write([]byte("erreur de compilation\n"))
			return bt.code
		}
		sortie.Write([]byte("Compilation...\nVersion : " + bt.version + "\n"))
		os.WriteFile(filepath.Join(bt.base, "out", "aibox-"+bt.version+"-x86_64.iso"), []byte("iso"), 0o644)
		return 0
	}
	espaceLibre = func(string) uint64 { return 100 }
	t.Cleanup(func() { espaceLibre = espaceLibreParDefaut })
	return bt
}

// espaceLibreParDefaut : la vraie mesure, pour rétablir la variable après les tests.
var espaceLibreParDefaut = espaceLibre

func TestBuildReussi(t *testing.T) {
	bt := nouveauBuild(t)
	var sortie, erreurs bytes.Buffer
	if code := build(bt.e, optionsBuild{ref: "main"}, &sortie, &erreurs); code != 0 {
		t.Fatalf("code = %d : %s", code, erreurs.String())
	}
	if !strings.Contains(sortie.String(), "ISO : ") {
		t.Errorf("sortie = %q", sortie.String())
	}
	b, _ := os.ReadFile(filepath.Join(bt.base, "dernier-build.json"))
	var r resultatBuild
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatalf("json : %v", err)
	}
	if r.Statut != "ok" || r.Version != bt.version || r.Commit != "abc1234" || r.Ref != "main" {
		t.Errorf("résultat = %+v", r)
	}
	if len(bt.appelsFlux) != 1 || !strings.HasPrefix(bt.appelsFlux[0], "nice -n 10 ") || !strings.HasSuffix(bt.appelsFlux[0], "build.sh") {
		t.Errorf("build.sh appelé : %v", bt.appelsFlux)
	}
	journal, _ := os.ReadFile(filepath.Join(bt.base, "build.log"))
	if !strings.Contains(string(journal), "Version : "+bt.version) {
		t.Errorf("journal sans la version : %q", journal)
	}
}

func TestBuildEchec(t *testing.T) {
	bt := nouveauBuild(t)
	bt.code = 2
	var sortie, erreurs bytes.Buffer
	if code := build(bt.e, optionsBuild{ref: "main"}, &sortie, &erreurs); code != 1 {
		t.Errorf("code = %d, attendu 1", code)
	}
	if !strings.Contains(erreurs.String(), "Échec du build") {
		t.Errorf("message = %q", erreurs.String())
	}
	b, _ := os.ReadFile(filepath.Join(bt.base, "dernier-build.json"))
	if !strings.Contains(string(b), `"statut": "echec"`) {
		t.Errorf("résultat = %s", b)
	}
}

func TestBuildFetchOnly(t *testing.T) {
	bt := nouveauBuild(t)
	var sortie, erreurs bytes.Buffer
	if code := build(bt.e, optionsBuild{ref: "main", fetchOnly: true}, &sortie, &erreurs); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if strings.TrimSpace(sortie.String()) != "abc1234" {
		t.Errorf("sortie = %q", sortie.String())
	}
	if len(bt.appelsFlux) != 0 {
		t.Error("build.sh ne doit pas tourner avec --fetch-only")
	}
}

func TestBuildEphemere(t *testing.T) {
	bt := nouveauBuild(t)
	os.WriteFile(filepath.Join(bt.e.racine, "proc", "self", "mountinfo"), nil, 0o644)
	var sortie, erreurs bytes.Buffer
	if code := build(bt.e, optionsBuild{ref: "main"}, &sortie, &erreurs); code != 1 {
		t.Errorf("code = %d, attendu 1", code)
	}
	if !strings.Contains(erreurs.String(), "mode éphémère") {
		t.Errorf("message = %q", erreurs.String())
	}
}

func TestBuildVerrouDejaPris(t *testing.T) {
	bt := nouveauBuild(t)
	autre, err := os.OpenFile(filepath.Join(bt.base, "verrou"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer autre.Close()
	if err := syscall.Flock(int(autre.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	var sortie, erreurs bytes.Buffer
	if code := build(bt.e, optionsBuild{ref: "main"}, &sortie, &erreurs); code != 1 {
		t.Errorf("code = %d, attendu 1", code)
	}
	if !strings.Contains(erreurs.String(), "déjà en cours") {
		t.Errorf("message = %q", erreurs.String())
	}
}

func TestBuildGardeFous(t *testing.T) {
	bt := nouveauBuild(t)
	os.Remove(filepath.Join(bt.base, "keys", "authorized_keys"))
	var sortie, erreurs bytes.Buffer
	if code := build(bt.e, optionsBuild{ref: "main"}, &sortie, &erreurs); code != 1 {
		t.Errorf("sans clés : code = %d", code)
	}

	bt = nouveauBuild(t)
	espaceLibre = func(string) uint64 { return 5 }
	erreurs.Reset()
	if code := build(bt.e, optionsBuild{ref: "main"}, &sortie, &erreurs); code != 1 || !strings.Contains(erreurs.String(), "Place insuffisante") {
		t.Errorf("sans place : code %d, %q", code, erreurs.String())
	}
}

func TestBuildImportRepreneDesSecrets(t *testing.T) {
	bt := nouveauBuild(t)
	depot := t.TempDir()
	os.MkdirAll(filepath.Join(depot, "keys"), 0o755)
	os.WriteFile(filepath.Join(depot, "keys", "tls.key"), []byte("cle"), 0o644)
	os.WriteFile(filepath.Join(depot, "keys", "README.md"), []byte("doc"), 0o644)
	os.MkdirAll(filepath.Join(depot, "work", "pkgcache"), 0o755)

	var sortie, erreurs bytes.Buffer
	if code := build(bt.e, optionsBuild{ref: "main", depotImport: depot}, &sortie, &erreurs); code != 0 {
		t.Fatalf("code = %d : %s", code, erreurs.String())
	}
	if _, err := os.Stat(filepath.Join(bt.base, "keys", "README.md")); err == nil {
		t.Error("README.md ne doit pas être repris")
	}
	info, err := os.Stat(filepath.Join(bt.base, "keys", "tls.key"))
	if err != nil {
		t.Fatalf("secret non repris : %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("droits du secret = %v, attendu 600", info.Mode().Perm())
	}
	if !strings.Contains(sortie.String(), "Secret repris : tls.key") {
		t.Errorf("sortie = %q", sortie.String())
	}
}

func TestDerniereVersion(t *testing.T) {
	if got := derniereVersion("Version : 1\nbla\nVersion : 2026.11.13.2100-abc\n"); got != "2026.11.13.2100-abc" {
		t.Errorf("version = %q", got)
	}
	if got := derniereVersion("rien"); got != "" {
		t.Errorf("version = %q", got)
	}
}
