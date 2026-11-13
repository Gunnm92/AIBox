package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	versionImage = "2026.11.12.0405-2658eae" // même commit que le dépôt : pas de commits neufs
	versionMajNeuve = "2026.11.13.2100-2658eae"
)

// banc : système factice pour aibox update (dépôt, ISO, ESP, faux outils).
type banc struct {
	t          *testing.T
	racine     string
	home       string
	iso        string // ISO factice (aibox/version, pkglist)
	racineISO  string // système de fichiers factice de l'ISO (/usr/local/bin/aibox)
	esp        string
	jeu        bool
	gitSortie  string
	llm        *httptest.Server
	appelsLLM  int
	appelsBuild []string
	appelsBoot  []string
	notifs     []string
	verifVersion string // version écrite dans l'ISO (pour simuler une image fausse)
	llmReponse string
	llmCode    int
}

func nouveauBanc(t *testing.T) *banc {
	t.Helper()
	// hermes doit « exister » pour que la notification parte (le test l'enregistre).
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "hermes"), []byte("#!/bin/sh\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	b := &banc{t: t, racine: t.TempDir(), home: t.TempDir(), verifVersion: versionMajNeuve, llmCode: 200,
		llmReponse: `{"mettre_a_jour": false, "raison": "paquets mineurs", "paquets_importants": []}`}
	b.llm = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.appelsLLM++
		if b.llmCode != 200 {
			w.WriteHeader(b.llmCode)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{
			{"message": map[string]string{"content": b.llmReponse}}}})
	}))
	t.Cleanup(b.llm.Close)

	ecrire := func(chemin, contenu string) {
		p := filepath.Join(b.racine, chemin)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(contenu), 0o644)
	}
	ecrire("etc/aibox/auto-update.conf", "MODEL=\"auto\"\nNOTIFY_TO=\"discord\"\nLLM_URL=\""+b.llm.URL+"/v1\"\n")

	// Dépôt : profiledef.sh avec un droit à vérifier.
	build := "var/lib/aibox/state/build"
	ecrire(build+"/src/archiso/profiledef.sh", "file_permissions=(\n  [\"/usr/local/bin/aibox\"]=\"0:0:755\"\n)\n")
	ecrire(build+"/src/archiso/pacman.conf", "")
	os.MkdirAll(filepath.Join(b.racine, build, "src", ".git"), 0o755)

	// Image installée sur AIBOXBOOT.
	b.esp = t.TempDir()
	os.MkdirAll(filepath.Join(b.esp, "loader"), 0o755)
	os.WriteFile(filepath.Join(b.esp, "loader", "loader.conf"), []byte("default aibox-"+versionImage+".conf\n"), 0o644)
	os.MkdirAll(filepath.Join(b.esp, "aibox-"+versionImage), 0o755)
	os.WriteFile(filepath.Join(b.esp, "aibox-"+versionImage, "pkglist.x86_64.txt"), []byte("mesa 26.0.0-1\nfiglet 2.2.5-1\n"), 0o644)

	// ISO nouvelle : version, pkglist, système factice avec aibox installé.
	b.iso = t.TempDir()
	os.MkdirAll(filepath.Join(b.iso, "aibox"), 0o755)
	os.WriteFile(filepath.Join(b.iso, "aibox", "pkglist.x86_64.txt"), []byte("mesa 26.1.0-1\nfiglet 2.2.5-1\n"), 0o644)
	os.WriteFile(filepath.Join(b.iso, "aibox", "version"), []byte(versionMajNeuve+"\n"), 0o644)
	b.racineISO = t.TempDir()
	os.MkdirAll(filepath.Join(b.racineISO, "usr", "local", "bin"), 0o755)
	os.WriteFile(filepath.Join(b.racineISO, "usr", "local", "bin", "aibox"), []byte("x"), 0o755)
	os.Chmod(filepath.Join(b.racineISO, "usr", "local", "bin", "aibox"), 0o755)

	// Image construite : fichier ISO et lien aibox-latest.iso.
	out := filepath.Join(b.racine, build, "out")
	os.MkdirAll(out, 0o755)
	os.WriteFile(filepath.Join(out, "aibox-"+versionMajNeuve+"-x86_64.iso"), []byte("iso"), 0o644)
	os.Symlink("aibox-"+versionMajNeuve+"-x86_64.iso", filepath.Join(out, "aibox-latest.iso"))
	return b
}

// env construit l'environnement factice : les appels sont enregistrés.
func (b *banc) env() env {
	b.t.Helper()
	e := env{racine: b.racine, home: b.home}
	e.lancer = func(_ []string, nom string, args ...string) (string, error) {
		switch nom {
		case "pgrep":
			if b.jeu {
				return "123\n", nil
			}
			return "", errors.New("exit 1")
		case "blkid":
			return "/dev/sdb2\n", nil
		case "vercmp":
			if args[0] == args[1] {
				return "0\n", nil
			}
			if versionAvant(args[0], args[1]) {
				return "-1\n", nil
			}
			return "1\n", nil
		case "hermes":
			b.notifs = append(b.notifs, args[len(args)-1])
			return "", nil
		case "pacman":
			if len(args) > 0 && args[len(args)-1] == "-Sl" {
				return "core mesa 26.1.0-1\nextra figlet 2.2.5-1\n", nil
			}
			return "", nil
		case "sudo":
			return b.sudo(args)
		}
		return "", nil
	}
	e.combine = func(vars []string, nom string, args ...string) string {
		out, _ := e.lancer(vars, nom, args...)
		return out
	}
	e.flux = func(_ []string, sortie io.Writer, nom string, args ...string) int {
		if nom == "sudo" && len(args) >= 3 && args[1] == "build" {
			b.appelsBuild = append(b.appelsBuild, strings.Join(args, " "))
			fmt.Fprintf(sortie, "Compilation…\nVersion : %s\n", versionMajNeuve)
			return 0
		}
		if nom == "sudo" && len(args) >= 2 && args[1] == "boot" {
			b.appelsBoot = append(b.appelsBoot, strings.Join(args, " "))
			return 0
		}
		return 0
	}
	return e
}

// sudo simule les commandes lancées en root.
func (b *banc) sudo(args []string) (string, error) {
	switch args[0] {
	case "aibox":
		if len(args) > 1 && args[1] == "build" && len(args) > 2 && args[2] == "--fetch-only" {
			return "2658eae\n", nil
		}
		return "", nil
	case "mount":
		cible := args[len(args)-1]
		opts := strings.Join(args, " ")
		var source string
		switch {
		case strings.Contains(opts, "erofs"):
			source = filepath.Join(b.racineISO)
		case strings.Contains(opts, "loop,ro"):
			source = b.iso
		default:
			source = b.esp
		}
		os.Remove(cible)
		return "", os.Symlink(source, cible)
	case "umount":
		cible := args[len(args)-1]
		if info, err := os.Lstat(cible); err == nil && info.Mode()&os.ModeSymlink != 0 {
			os.Remove(cible)
		}
		return "", nil
	case "cat":
		b, err := os.ReadFile(args[1])
		return string(b), err
	case "git":
		return b.gitSortie, nil
	case "stat":
		info, err := os.Stat(args[len(args)-1])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%o\n", info.Mode().Perm()), nil
	case "rm":
		os.RemoveAll(args[len(args)-1])
		return "", nil
	}
	return "", nil
}

// lancer exécute aibox update avec les options données, et renvoie le code et la sortie.
func (b *banc) lancer(args ...string) (int, string) {
	b.t.Helper()
	var sortie, erreurs bytes.Buffer
	c := &contexteUpdate{
		e: b.env(), sortie: &sortie, erreurs: &erreurs,
		client:     &http.Client{Timeout: 10 * time.Second},
		maintenant: func() time.Time { return time.Date(2026, 11, 13, 21, 0, 0, 0, time.UTC) },
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			c.o.dryRun = true
		case "--no-install":
			c.o.noInstall = true
		case "--force":
			c.o.force = true
		}
	}
	code := miseAJour(c, args)
	return code, erreurs.String()
}

func TestUpdateJeuEnCours(t *testing.T) {
	b := nouveauBanc(t)
	b.jeu = true
	code, log := b.lancer("--dry-run")
	if code != 0 || !strings.Contains(log, "Un jeu tourne") {
		t.Errorf("code %d, journal %q", code, log)
	}
}

func TestUpdateDryRunAvisDuLLMNon(t *testing.T) {
	b := banqueAvec(t, versionImage)
	code, log := b.lancer("--dry-run")
	if code != 0 {
		t.Fatalf("code = %d : %s", code, log)
	}
	if b.appelsLLM != 1 || !strings.Contains(log, "Décision : false") {
		t.Errorf("LLM appelé %d fois ; journal : %s", b.appelsLLM, log)
	}
	if len(b.appelsBuild) != 0 {
		t.Error("aucun build attendu quand le LLM dit non")
	}
	dec, _ := os.ReadFile(filepath.Join(b.home, ".local", "state", "aibox", "auto-update", "derniere-decision.json"))
	if !strings.Contains(string(dec), `"mettre_a_jour": false`) || !strings.Contains(string(dec), `"image": "`+versionImage+`"`) {
		t.Errorf("derniere-decision.json = %s", dec)
	}
}

func TestUpdateDryRunAvisDuLLMOui(t *testing.T) {
	b := banqueAvec(t, versionImage)
	b.llmReponse = `<think>réflexion</think>{"mettre_a_jour": true, "raison": "mesa", "paquets_importants": ["mesa 26.0.0-1 → 26.1.0-1"]}`
	code, log := b.lancer("--dry-run")
	if code != 0 || !strings.Contains(log, "--dry-run : pas de build.") {
		t.Errorf("code %d : %s", code, log)
	}
	if len(b.appelsBuild) != 0 {
		t.Error("--dry-run ne doit pas construire")
	}
}

func TestUpdateConstruitEtVerifie(t *testing.T) {
	b := banqueAvec(t, versionImage)
	b.llmReponse = `{"mettre_a_jour": true, "raison": "mesa", "paquets_importants": []}`
	code, log := b.lancer("--no-install")
	if code != 0 {
		t.Fatalf("code = %d : %s", code, log)
	}
	if len(b.appelsBuild) != 1 || !strings.Contains(log, "Image "+versionMajNeuve+" vérifiée.") {
		t.Errorf("build = %v, journal : %s", b.appelsBuild, log)
	}
	if len(b.appelsBoot) != 0 {
		t.Error("--no-install ne doit pas installer")
	}
}

func TestUpdateInstalleEtNotifie(t *testing.T) {
	b := banqueAvec(t, versionImage)
	b.llmReponse = `{"mettre_a_jour": true, "raison": "mesa", "paquets_importants": []}`
	code, log := b.lancer()
	if code != 0 {
		t.Fatalf("code = %d : %s", code, log)
	}
	if len(b.appelsBoot) != 1 || !strings.Contains(b.appelsBoot[0], "--iso") {
		t.Errorf("installation = %v", b.appelsBoot)
	}
	if len(b.notifs) == 0 || !strings.Contains(b.notifs[len(b.notifs)-1], "Nouvelle image installée : "+versionMajNeuve) {
		t.Errorf("notifications = %q", b.notifs)
	}
}

func TestUpdateNouveauxCommitsSansLLM(t *testing.T) {
	// Image d'un autre commit : les commits poussés sont construits sans demander au LLM.
	b := banqueAvec(t, "2026.11.12.0405-7fa8dfc")
	b.gitSortie = "2658eae Séparer la mise à jour\n"
	code, log := b.lancer("--no-install")
	if code != 0 {
		t.Fatalf("code = %d : %s", code, log)
	}
	if b.appelsLLM != 0 {
		t.Errorf("le LLM ne doit pas être appelé (%d)", b.appelsLLM)
	}
	if len(b.appelsBuild) != 1 {
		t.Errorf("build = %v", b.appelsBuild)
	}
}

func TestUpdateVerificationRefuseUneImageFausse(t *testing.T) {
	b := banqueAvec(t, versionImage)
	b.llmReponse = `{"mettre_a_jour": true, "raison": "mesa", "paquets_importants": []}`
	b.verifVersion = "autre"
	os.WriteFile(filepath.Join(b.iso, "aibox", "version"), []byte("2026.00.00.0000-faux\n"), 0o644)
	code, log := b.lancer("--no-install")
	if code != 1 || !strings.Contains(log, "refusée à la vérification") {
		t.Errorf("code %d, journal : %s", code, log)
	}
	if len(b.appelsBoot) != 0 {
		t.Error("une image refusée ne doit jamais être installée")
	}
}

func TestUpdateLLMIndisponible(t *testing.T) {
	b := banqueAvec(t, versionImage)
	b.llmCode = 500
	code, log := b.lancer("--dry-run")
	if code != 1 || !strings.Contains(log, "le LLM ne répond pas") {
		t.Errorf("code %d, journal : %s", code, log)
	}
}

func TestUpdateRienDeNeuf(t *testing.T) {
	b := banqueAvec(t, versionImage)
	os.WriteFile(filepath.Join(b.esp, "aibox-"+versionImage, "pkglist.x86_64.txt"), []byte("mesa 26.1.0-1\n"), 0o644)
	code, log := b.lancer("--dry-run")
	if code != 0 || !strings.Contains(log, "Rien de nouveau dans les dépôts.") || b.appelsLLM != 0 {
		t.Errorf("code %d, LLM %d, journal : %s", code, b.appelsLLM, log)
	}
}

// banqueAvec : banc dont l'image installée a la version donnée.
func banqueAvec(t *testing.T, version string) *banc {
	t.Helper()
	b := nouveauBanc(t)
	os.RemoveAll(filepath.Join(b.esp, "aibox-"+versionImage))
	os.Remove(filepath.Join(b.esp, "loader", "loader.conf"))
	os.WriteFile(filepath.Join(b.esp, "loader", "loader.conf"), []byte("default aibox-"+version+".conf\n"), 0o644)
	os.MkdirAll(filepath.Join(b.esp, "aibox-"+version), 0o755)
	os.WriteFile(filepath.Join(b.esp, "aibox-"+version, "pkglist.x86_64.txt"), []byte("mesa 26.0.0-1\nfiglet 2.2.5-1\n"), 0o644)
	return b
}

// Conteneurs : une mise à jour réussie et un échec, notifiés ensemble.
func TestUpdatePodmanMiseAJourEtEchec(t *testing.T) {
	b := banqueAvec(t, versionImage)
	e := b.env()
	e.flux = func(_ []string, sortie io.Writer, nom string, args ...string) int {
		if nom == "podman" {
			sortie.Write([]byte("litellm-pod.service\tghcr.io/berriai/litellm:main-stable\ttrue\n" +
				"searxng.service\tdocker.io/searxng/searxng:latest\tfailed\n"))
		}
		return 0
	}
	var sortie, erreurs bytes.Buffer
	c := &contexteUpdate{
		e: e, o: optionsUpdate{podman: true}, sortie: &sortie, erreurs: &erreurs,
		client:     &http.Client{Timeout: 10 * time.Second},
		maintenant: func() time.Time { return time.Date(2026, 11, 13, 21, 0, 0, 0, time.UTC) },
	}
	if code := miseAJour(c, []string{"--podman"}); code != 0 {
		t.Fatalf("code = %d : %s", code, erreurs.String())
	}
	if len(b.notifs) != 1 {
		t.Fatalf("notifications = %q", b.notifs)
	}
	for _, attendu := range []string{"Conteneurs Podman, mis à jour :", "litellm", "Problème :", "searxng"} {
		if !strings.Contains(b.notifs[0], attendu) {
			t.Errorf("notification sans %q : %q", attendu, b.notifs[0])
		}
	}
}
