package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nasEnv : système temporaire avec /etc/aibox, /var/lib/aibox/state (monté) et
// un systemd-mount simulé (les appels sont enregistrés).
func nasEnv(t *testing.T, mountinfo string) (env, *[]string) {
	t.Helper()
	racine := t.TempDir()
	os.MkdirAll(filepath.Join(racine, "etc", "aibox"), 0o755)
	os.MkdirAll(filepath.Join(racine, "var", "lib", "aibox", "state"), 0o755)
	os.MkdirAll(filepath.Join(racine, "proc", "self"), 0o755)
	os.WriteFile(filepath.Join(racine, "proc", "self", "mountinfo"), []byte(mountinfo), 0o644)
	var appels []string
	e := env{racine: racine, home: t.TempDir()}
	e.lancer = func(_ []string, nom string, args ...string) (string, error) {
		appels = append(appels, nom+" "+strings.Join(args, " "))
		return "", nil
	}
	return e, &appels
}

// montageDonnees : le disque de données est monté (requis pour ajouter un partage).
const montageDonnees = "36 35 0:31 / /var/lib/aibox/state rw - btrfs /dev/nvme0n1p2[/@data] rw\n"

func TestNasListeEtat(t *testing.T) {
	e, _ := nasEnv(t, montageDonnees+
		"40 35 0:40 / /mnt/nas rw - cifs //nas/docs rw\n"+
		"41 35 0:41 / /mnt/autre rw - autofs systemd-1 rw\n")
	os.WriteFile(filepath.Join(e.racine, "etc", "aibox", "nas-mounts"),
		[]byte("//nas/docs /mnt/nas\n//nas/photos /mnt/photos\n//nas/vrac /mnt/autre\n"), 0o644)
	var sortie bytes.Buffer
	nasList(e, &sortie)
	s := sortie.String()
	for _, attendu := range []string{"//nas/docs", "connecté", "non monté", "prêt (monté au premier accès)"} {
		if !strings.Contains(s, attendu) {
			t.Errorf("liste sans %q :\n%s", attendu, s)
		}
	}
}

func TestNasUtilisateurPrimeSurImage(t *testing.T) {
	e, _ := nasEnv(t, montageDonnees)
	os.WriteFile(filepath.Join(e.racine, "etc", "aibox", "nas-mounts"), []byte("//image/x /mnt/x\n"), 0o644)
	etat := filepath.Join(e.racine, "var", "lib", "aibox", "state", "nas")
	os.MkdirAll(etat, 0o700)
	os.WriteFile(filepath.Join(etat, "mounts"), []byte("//perso/y /mnt/x\n"), 0o600)
	lignes := lignesNas(e)
	if len(lignes) != 1 || lignes[0].src != "//perso/y" {
		t.Errorf("lignes = %+v, attendu seulement le partage de l'utilisateur", lignes)
	}
	if !strings.HasSuffix(lignes[0].cred, "perso.cred") {
		t.Errorf("identifiants par défaut = %q", lignes[0].cred)
	}
}

func TestNasAjoutDemandeLesIdentifiantsUneFois(t *testing.T) {
	e, appels := nasEnv(t, montageDonnees)
	var sortie, erreurs bytes.Buffer
	entree := bufio.NewReader(strings.NewReader("alice\nsecret\n"))
	if code := nasAjouter(e, "//nas/docs", "/mnt/docs", entree, &sortie, &erreurs); code != 0 {
		t.Fatalf("code = %d : %s", code, erreurs.String())
	}
	cred := filepath.Join(e.racine, "var", "lib", "aibox", "state", "nas", "nas.cred")
	b, err := os.ReadFile(cred)
	if err != nil {
		t.Fatalf("identifiants non écrits : %v", err)
	}
	if string(b) != "username=alice\npassword=secret\n" {
		t.Errorf("identifiants = %q", b)
	}
	if info, _ := os.Stat(cred); info.Mode().Perm() != 0o600 {
		t.Errorf("droits des identifiants = %v, attendu 600", info.Mode().Perm())
	}
	conf, _ := os.ReadFile(filepath.Join(e.racine, "var", "lib", "aibox", "state", "nas", "mounts"))
	if string(conf) != "//nas/docs /mnt/docs\n" {
		t.Errorf("configuration = %q", conf)
	}
	if len(*appels) != 1 || !strings.Contains((*appels)[0], "systemd-mount") || !strings.Contains((*appels)[0], "credentials=") {
		t.Errorf("montage = %v", *appels)
	}

	// Deuxième ajout sur le même hôte : pas de nouvelle demande d'identifiants.
	entree = bufio.NewReader(strings.NewReader("ne-doit-pas-servir\n"))
	nasAjouter(e, "//nas/photos", "/mnt/photos", entree, &sortie, &erreurs)
	b, _ = os.ReadFile(cred)
	if string(b) != "username=alice\npassword=secret\n" {
		t.Errorf("identifiants réécrits : %q", b)
	}
}

func TestNasAjoutRefuseLesSourcesMalformees(t *testing.T) {
	e, _ := nasEnv(t, montageDonnees)
	for _, mauvaise := range []string{"nas/docs", "//nas", "smb://nas/docs"} {
		var sortie, erreurs bytes.Buffer
		if code := nasAjouter(e, mauvaise, "/mnt/x", bufio.NewReader(strings.NewReader("")), &sortie, &erreurs); code != 1 {
			t.Errorf("%q : code = %d, attendu 1", mauvaise, code)
		}
	}
}

func TestNasAjoutSansDisqueDeDonnees(t *testing.T) {
	e, _ := nasEnv(t, "") // pas de montage de /var/lib/aibox/state : mode éphémère
	var sortie, erreurs bytes.Buffer
	if code := nasAjouter(e, "//nas/docs", "/mnt/docs", bufio.NewReader(strings.NewReader("")), &sortie, &erreurs); code != 1 {
		t.Errorf("code = %d, attendu 1", code)
	}
	if !strings.Contains(erreurs.String(), "mode éphémère") {
		t.Errorf("message = %q", erreurs.String())
	}
}

func TestNasRetirerSeulementUnAjout(t *testing.T) {
	e, appels := nasEnv(t, montageDonnees)
	etat := filepath.Join(e.racine, "var", "lib", "aibox", "state", "nas")
	os.MkdirAll(etat, 0o700)
	os.WriteFile(filepath.Join(etat, "mounts"), []byte("//nas/docs /mnt/docs\n"), 0o600)
	var sortie, erreurs bytes.Buffer
	if code := nasRetirer(e, "/mnt/docs", &sortie, &erreurs); code != 0 {
		t.Fatalf("retrait : code = %d : %s", code, erreurs.String())
	}
	if conf, _ := os.ReadFile(filepath.Join(etat, "mounts")); strings.Contains(string(conf), "/mnt/docs") {
		t.Errorf("ligne non retirée : %q", conf)
	}
	if len(*appels) != 1 || !strings.Contains((*appels)[0], "--umount /mnt/docs") {
		t.Errorf("démontage = %v", *appels)
	}
	erreurs.Reset()
	if code := nasRetirer(e, "/mnt/image", &sortie, &erreurs); code != 1 {
		t.Errorf("partage de l'image : code = %d, attendu 1", code)
	}
}

func TestNasMonterToutIgnoreLesMontes(t *testing.T) {
	e, appels := nasEnv(t, montageDonnees+"40 35 0:40 / /mnt/deja rw - cifs //nas/deja rw\n")
	os.WriteFile(filepath.Join(e.racine, "etc", "aibox", "nas-mounts"),
		[]byte("//nas/deja /mnt/deja\n//nas/neuf /mnt/neuf\n"), 0o644)
	os.WriteFile(filepath.Join(e.racine, "etc", "aibox", "nas.cred"), []byte("username=u\npassword=p\n"), 0o600)
	var sortie bytes.Buffer
	nasMonterTout(e, &sortie)
	if len(*appels) != 1 || !strings.Contains((*appels)[0], "/mnt/neuf") {
		t.Errorf("appels = %v, attendu un seul montage de /mnt/neuf", *appels)
	}
}
