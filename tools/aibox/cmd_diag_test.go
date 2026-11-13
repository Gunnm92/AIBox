package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// diagTest : système temporaire, commandes simulées (sortie fixe par commande).
func diagTest(t *testing.T, mountinfo string, blkid map[string]string) env {
	t.Helper()
	racine := t.TempDir()
	os.MkdirAll(filepath.Join(racine, "proc", "self"), 0o755)
	os.WriteFile(filepath.Join(racine, "proc", "self", "mountinfo"), []byte(mountinfo), 0o644)
	os.WriteFile(filepath.Join(racine, "proc", "cmdline"), []byte("quiet splash\n"), 0o644)
	os.MkdirAll(filepath.Join(racine, "home", "default", ".config", "aibox"), 0o755)
	os.WriteFile(filepath.Join(racine, "home", "default", ".config", "aibox", "tv-mode"), []byte("gamescope\n"), 0o644)
	os.MkdirAll(filepath.Join(racine, "sys", "class", "drm", "card1-DP-1"), 0o755)
	os.WriteFile(filepath.Join(racine, "sys", "class", "drm", "card1-DP-1", "status"), []byte("connected\n"), 0o644)
	os.WriteFile(filepath.Join(racine, "sys", "class", "drm", "card1-DP-1", "modes"), []byte("3840x2160\n1920x1080\n"), 0o644)

	e := env{racine: racine, home: filepath.Join(racine, "home", "default")}
	e.combine = func(_ []string, nom string, args ...string) string {
		if nom == "blkid" {
			return blkid[args[1]]
		}
		if nom == "journalctl" {
			var b strings.Builder
			for i := 1; i <= 100; i++ {
				b.WriteString("ligne " + strconv.Itoa(i) + "\n")
			}
			return b.String() + "amdgpu: DMUB initialisé\n"
		}
		return nom + " ok\n"
	}
	e.lancer = func(_ []string, nom string, args ...string) (string, error) {
		return "", nil
	}
	e.executer = nil
	return e
}

func TestDiagRapportSections(t *testing.T) {
	e := diagTest(t, "", map[string]string{})
	var sortie bytes.Buffer
	diag(e, time.Date(2026, 11, 13, 21, 30, 0, 0, time.Local), &sortie)

	rapports, _ := filepath.Glob(filepath.Join(e.racine, "run", "aibox-diag", "aibox-diag-*.txt"))
	if len(rapports) != 1 {
		t.Fatalf("rapports = %v", rapports)
	}
	b, _ := os.ReadFile(rapports[0])
	rapport := string(b)
	for _, titre := range []string{"Système", "Session TV (système)", "Journal aibox-tv", "Sessions logind",
		"Processus graphiques", "Sockets Wayland", "Écrans (DRM)", "Noyau : amdgpu / drm",
		"Services en échec", "Réseau", "Disques", "Mémoire", "Journal complet du démarrage"} {
		if !strings.Contains(rapport, "===== "+titre+" =====") {
			t.Errorf("section absente : %s", titre)
		}
	}
	for _, attendu := range []string{"mode TV : gamescope", "3840x2160", "connected", "amdgpu: DMUB", "quiet splash"} {
		if !strings.Contains(rapport, attendu) {
			t.Errorf("rapport sans %q", attendu)
		}
	}
	if !strings.Contains(sortie.String(), "ni clé Ventoy ni partition AIBOXBOOT") {
		t.Errorf("annonce absente : %q", sortie.String())
	}
}

func TestDiagCopieSurLaCleVentoyDejaMontee(t *testing.T) {
	cle := t.TempDir()
	mountinfo := "50 35 0:50 / " + cle + " rw - vfat /dev/sdb1 rw\n"
	e := diagTest(t, mountinfo, map[string]string{"Ventoy": "/dev/sdb1\n"})
	var sortie bytes.Buffer
	diag(e, time.Date(2026, 11, 13, 21, 30, 0, 0, time.Local), &sortie)

	copies, _ := filepath.Glob(filepath.Join(cle, "aibox-diag", "aibox-diag-*.txt"))
	if len(copies) != 1 {
		t.Fatalf("copie absente sur la clé : %v", copies)
	}
	if !strings.Contains(sortie.String(), "Copié sur Ventoy : aibox-diag/") {
		t.Errorf("annonce = %q", sortie.String())
	}
}

func TestDiagSansPartitionNiCle(t *testing.T) {
	e := diagTest(t, "", map[string]string{})
	if dest := copierRapport(e, filepath.Join(e.racine, "rapport.txt"), "rapport.txt"); dest != "" {
		t.Errorf("copie inattendue : %q", dest)
	}
}

func TestFiltrerEtDernieresLignes(t *testing.T) {
	if got := filtrer("a\namdgpu x\nb\nDRM y\n", `(?i)amdgpu|drm`, 60); got != "amdgpu x\nDRM y\n" {
		t.Errorf("filtrer = %q", got)
	}
	if got := dernieresLignes("1\n2\n3\n", 2); got != "2\n3\n" {
		t.Errorf("dernieresLignes = %q", got)
	}
}
