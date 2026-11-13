package main

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isoFactice : une ISO aibox minimale (grub.cfg, version, noyau, initramfs, système
// et sa somme SHA-512), sous un dossier temporaire. Renvoie le dossier.
func isoFactice(t *testing.T, version string) string {
	t.Helper()
	src := t.TempDir()
	ecrire := func(rel, contenu string) {
		p := filepath.Join(src, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(contenu), 0o644)
	}
	ecrire("boot/grub/grub.cfg", "menuentry \"aibox\" {\n"+
		"\tlinux /aibox/boot/vmlinuz-linux-cachyos archisobasedir=aibox archisolabel=AIBOX copytoram=y\n"+
		"\tinitrd /aibox/boot/initramfs-linux-cachyos.img\n}\n")
	ecrire("aibox/version", version+"\n")
	ecrire("aibox/boot/vmlinuz-linux-cachyos", "noyau")
	ecrire("aibox/boot/initramfs-linux-cachyos.img", "ram")
	systeme := []byte("systeme de fichiers")
	ecrire("aibox/x86_64/airootfs.erofs", string(systeme))
	somme := sha512.Sum512(systeme)
	ecrire("aibox/x86_64/airootfs.sha512", hex.EncodeToString(somme[:])+"  airootfs.erofs\n")
	return src
}

// espFictive : partition de démarrage vide (dossier loader/entries) et environnement.
func espFictive(t *testing.T) (env, string) {
	t.Helper()
	esp := t.TempDir()
	os.MkdirAll(filepath.Join(esp, "loader", "entries"), 0o755)
	return env{racine: "/", home: t.TempDir()}, esp
}

func installer(t *testing.T, src, esp, running string, keep int) (int, string, string) {
	t.Helper()
	img, err := lireImage(src)
	if err != nil {
		t.Fatalf("lecture de l'ISO : %v", err)
	}
	var sortie, erreurs bytes.Buffer
	code := installerDansEsp(env{racine: "/"}, esp, img, "UUID-TEST", running, keep, "/dev/fake1", &sortie, &erreurs)
	return code, sortie.String(), erreurs.String()
}

const versionNeuve = "2026.11.13.2100-abc1234"

func TestBootInstallationPropre(t *testing.T) {
	_, esp := espFictive(t)
	src := isoFactice(t, versionNeuve)
	code, sortie, erreurs := installer(t, src, esp, "", 2)
	if code != 0 {
		t.Fatalf("code = %d : %s", code, erreurs)
	}
	nom := "aibox-" + versionNeuve
	if _, err := os.Stat(filepath.Join(esp, nom, "x86_64", "airootfs.erofs")); err != nil {
		t.Errorf("image absente de l'ESP : %v", err)
	}
	entree, _ := os.ReadFile(filepath.Join(esp, "loader", "entries", nom+".conf"))
	for _, attendu := range []string{
		"linux    /" + nom + "/boot/vmlinuz-linux-cachyos",
		"initrd   /" + nom + "/boot/initramfs-linux-cachyos.img",
		"options  archisobasedir=" + nom + " archisodevice=UUID=UUID-TEST",
		"copytoram=y",
	} {
		if !strings.Contains(string(entree), attendu) {
			t.Errorf("entrée sans %q :\n%s", attendu, entree)
		}
	}
	dep, _ := os.ReadFile(filepath.Join(esp, "loader", "entries", nom+"-noram.conf"))
	if !strings.Contains(string(dep), "copytoram=n") || strings.Contains(string(dep), "copytoram=y") {
		t.Errorf("entrée sans copie en RAM incorrecte :\n%s", dep)
	}
	menu, _ := os.ReadFile(filepath.Join(esp, "loader", "loader.conf"))
	if !strings.Contains(string(menu), "default "+nom+".conf") {
		t.Errorf("loader.conf = %q", menu)
	}
	if !strings.Contains(sortie, "Par défaut au prochain démarrage : aibox "+versionNeuve) {
		t.Errorf("sortie = %q", sortie)
	}
}

func TestBootSommeCorrompueNeLaisseRien(t *testing.T) {
	_, esp := espFictive(t)
	src := isoFactice(t, versionNeuve)
	os.WriteFile(filepath.Join(src, "aibox", "x86_64", "airootfs.erofs"), []byte("altere"), 0o644)
	code, _, erreurs := installer(t, src, esp, "", 2)
	if code != 1 || !strings.Contains(erreurs, "somme SHA-512") {
		t.Errorf("code = %d, erreurs = %q", code, erreurs)
	}
	entrees, _ := os.ReadDir(esp)
	for _, d := range entrees {
		if strings.HasPrefix(d.Name(), "aibox-") {
			t.Errorf("reste sur l'ESP après échec : %s", d.Name())
		}
	}
}

func TestBootGardeLesDernieresVersions(t *testing.T) {
	_, esp := espFictive(t)
	for _, v := range []string{"aibox-2026.11.10.0405-aaa1111", "aibox-2026.11.11.0405-bbb2222"} {
		os.MkdirAll(filepath.Join(esp, v), 0o755)
		os.WriteFile(filepath.Join(esp, "loader", "entries", v+".conf"), []byte("x"), 0o644)
	}
	src := isoFactice(t, versionNeuve)
	code, sortie, erreurs := installer(t, src, esp, "", 2)
	if code != 0 {
		t.Fatalf("code = %d : %s", code, erreurs)
	}
	if _, err := os.Stat(filepath.Join(esp, "aibox-2026.11.10.0405-aaa1111")); err == nil {
		t.Error("la plus ancienne version aurait dû être retirée (--keep 2)")
	}
	if _, err := os.Stat(filepath.Join(esp, "aibox-2026.11.11.0405-bbb2222")); err != nil {
		t.Error("la version précédente doit rester (menu de démarrage)")
	}
	if _, err := os.Stat(filepath.Join(esp, "loader", "entries", "aibox-2026.11.10.0405-aaa1111.conf")); err == nil {
		t.Error("l'entrée de menu de la version retirée doit l'être aussi")
	}
	if !strings.Contains(sortie, "Ancienne version retirée : aibox-2026.11.10.0405-aaa1111") {
		t.Errorf("sortie = %q", sortie)
	}
}

func TestBootNeRetireJamaisLaVersionEnCours(t *testing.T) {
	_, esp := espFictive(t)
	ancienne := "aibox-2026.11.10.0405-aaa1111"
	os.MkdirAll(filepath.Join(esp, ancienne), 0o755)
	src := isoFactice(t, versionNeuve)
	code, _, erreurs := installer(t, src, esp, ancienne, 1)
	if code != 0 {
		t.Fatalf("code = %d : %s", code, erreurs)
	}
	if _, err := os.Stat(filepath.Join(esp, ancienne)); err != nil {
		t.Error("la version en cours ne doit jamais être retirée")
	}
}

func TestBootRefuseUnNomDeVersionSuspect(t *testing.T) {
	_, esp := espFictive(t)
	src := isoFactice(t, "../../etc")
	img, err := lireImage(src)
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	var sortie, erreurs bytes.Buffer
	if code := installerDansEsp(env{racine: "/"}, esp, img, "UUID", "", 2, "/dev/fake1", &sortie, &erreurs); code != 1 {
		t.Errorf("code = %d, attendu 1", code)
	}
}

func TestBootIsoNonAibox(t *testing.T) {
	if _, err := lireImage(t.TempDir()); err == nil || !strings.Contains(err.Error(), "Ce n'est pas une ISO aibox") {
		t.Errorf("erreur = %v", err)
	}
}

func TestBootUsageDuKeep(t *testing.T) {
	var sortie, erreurs bytes.Buffer
	if code := cmdBoot([]string{"--keep", "zero"}, &sortie, &erreurs); code != 1 {
		t.Errorf("code = %d, attendu 1", code)
	}
}
