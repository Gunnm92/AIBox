package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

const usageUpdate = `Usage : aibox update [--podman] [--dry-run] [--no-install] [--force] [--pkglist FICHIER]
  aibox update                 mise à jour du système : image décidée par le LLM local, ou
                               nouveaux commits poussés (build, vérification, installation)
  aibox update --podman        conteneurs Podman seulement (podman auto-update)
  --dry-run     comparaison et avis du LLM seulement (conteneurs : liste sans les faire)
  --no-install  construit et vérifie l'image, sans l'installer
  --force       sans l'avis du LLM (mise à jour même sans paquet nouveau)
  --pkglist F   liste « nom version » de référence à la place de l'image installée`

// optionsUpdate : options de aibox update.
type optionsUpdate struct {
	podman, dryRun, noInstall, force bool
	pkglist                          string
}

// reglagesUpdate : réglages de auto-update.conf, avec leurs valeurs par défaut.
type reglagesUpdate struct {
	buildDir, buildRef, llmURL, modele, notifyTo, llmKey string
}

// contexteUpdate : ce qu'il faut à une mise à jour pour parler au système et au journal.
type contexteUpdate struct {
	e        env
	o        optionsUpdate
	r        reglagesUpdate
	etat     string
	journal  string
	sortie   io.Writer
	erreurs  io.Writer
	client   *http.Client
	maintenant func() time.Time
}

// cmdUpdate : « aibox update », mise à jour du système ou des conteneurs.
func cmdUpdate(args []string, sortie, erreurs io.Writer) int {
	o := optionsUpdate{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--podman":
			o.podman = true
		case "--dry-run":
			o.dryRun = true
		case "--no-install":
			o.noInstall = true
		case "--force":
			o.force = true
		case "--pkglist":
			if i+1 >= len(args) {
				fmt.Fprintln(erreurs, usageUpdate)
				return 2
			}
			o.pkglist = args[i+1]
			i++
		default:
			fmt.Fprintln(erreurs, usageUpdate)
			return 2
		}
	}
	if o.podman && (o.noInstall || o.force || o.pkglist != "") {
		fmt.Fprintln(erreurs, "--podman ne se combine qu'avec --dry-run.")
		return 2
	}
	e := envReel()
	ctx := &contexteUpdate{
		e: e, o: o, sortie: sortie, erreurs: erreurs,
		client:     &http.Client{Timeout: 1200 * time.Second},
		maintenant: time.Now,
	}
	return miseAJour(ctx, args)
}

// miseAJour : déroulé complet de aibox update (même ordre que le script).
func miseAJour(c *contexteUpdate, args []string) int {
	c.etat = filepath.Join(c.e.home, ".local", "state", "aibox", "auto-update")
	if err := os.MkdirAll(c.etat, 0o755); err != nil {
		fmt.Fprintf(c.erreurs, "aibox update : %v\n", err)
		return 1
	}
	c.journal = filepath.Join(c.etat, "journal.log")
	c.ouvrirJournal(args)

	// Une seule exécution à la fois.
	verrou, err := os.OpenFile(filepath.Join(c.etat, "verrou"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		c.journaliser("aibox update : " + err.Error())
		return 1
	}
	defer verrou.Close()
	if err := syscall.Flock(int(verrou.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		c.journaliser("Déjà en cours.")
		return 0
	}

	c.lireReglages()

	// Garde-fou : un jeu Steam, ou un jeu Windows lancé par umu, est en cours.
	if _, err := c.e.lancer(nil, "pgrep", "-f", "[S]teamLaunch AppId=|[p]roton waitforexitandrun"); err == nil {
		c.journaliser("Un jeu tourne : mise à jour reportée à la prochaine fois.")
		return 0
	}

	if c.o.podman {
		c.conteneurs()
		return 0
	}
	return c.systeme()
}

// reglagesUpdate : auto-update.conf du système, puis de l'utilisateur (le second l'emporte).
func (c *contexteUpdate) lireReglages() {
	c.r = reglagesUpdate{buildDir: "/var/lib/aibox/state/build", buildRef: "main", modele: "auto", notifyTo: "discord"}
	valeurs := map[string]string{}
	for _, f := range []string{c.e.chemin("etc/aibox/auto-update.conf"), filepath.Join(c.e.home, ".config", "aibox", "auto-update.conf")} {
		for k, v := range lireEnv(f) {
			valeurs[k] = v
		}
	}
	if v := valeurs["BUILD_DIR"]; v != "" {
		c.r.buildDir = v
	}
	c.r.buildDir = c.e.chemin(c.r.buildDir)
	if v := valeurs["BUILD_REF"]; v != "" {
		c.r.buildRef = v
	}
	if v := valeurs["MODEL"]; v != "" {
		c.r.modele = v
	}
	if v := valeurs["NOTIFY_TO"]; v != "" {
		c.r.notifyTo = v
	}
	c.r.llmKey = valeurs["LLM_API_KEY"]
	c.r.llmURL = valeurs["LLM_URL"]
	if c.r.llmURL == "" {
		c.r.llmURL = valeurs["LEMONADE_URL"]
	}
	if c.r.llmURL == "" {
		c.r.llmURL = "http://127.0.0.1:4000/v1"
	}
}

// ouvrirJournal ajoute l'entête de l'exécution, puis ramène le journal à sa taille.
func (c *contexteUpdate) ouvrirJournal(args []string) {
	if lignes, err := lireLignes(c.journal); err == nil {
		gardees := gardeJournal(lignes)
		if len(gardees) != len(lignes) {
			os.WriteFile(c.journal, []byte(strings.Join(gardees, "\n")+"\n"), 0o644)
		}
	}
	c.journaliser("--- aibox update " + strings.Join(args, " "))
}

// journaliser : message sur stderr et dans le journal (disque de données).
func (c *contexteUpdate) journaliser(msg string) {
	fmt.Fprintln(c.erreurs, msg)
	f, err := os.OpenFile(c.journal, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", c.maintenant().Format("2006-01-02 15:04:05"), msg)
}

// notifier envoie un message sur le canal habituel (hermes send), s'il est installé.
func (c *contexteUpdate) notifier(msg string) {
	c.journaliser("Notification : " + msg)
	if _, err := exec.LookPath("hermes"); err != nil {
		c.journaliser("hermes absent : pas de notification.")
		return
	}
	if _, err := c.e.lancer(nil, "hermes", "send", "-q", "--to", c.r.notifyTo, "--subject", "aibox — mise à jour", msg); err != nil {
		c.journaliser("Notification impossible (hermes send).")
	}
}

// echec prévient et s'arrête.
func (c *contexteUpdate) echec(msg string) int {
	c.notifier("Échec : " + msg)
	return 1
}

// conteneurs : mise à jour des Quadlets Podman marqués AutoUpdate=registry.
func (c *contexteUpdate) conteneurs() {
	format := "{{.Unit}}\t{{.Image}}\t{{.Updated}}"
	var buf bytes.Buffer
	if c.o.dryRun {
		c.e.flux(nil, &buf, "podman", "auto-update", "--dry-run", "--format", format)
		c.journaliser("Conteneurs (--dry-run) :\n" + buf.String())
		return
	}
	rc := c.e.flux(nil, &buf, "podman", "auto-update", "--format", format)
	sortie := buf.String()
	c.journaliser("Conteneurs :\n" + sortie)

	var maj, pb []string
	for _, l := range strings.Split(sortie, "\n") {
		champs := strings.Split(l, "\t")
		if len(champs) != 3 {
			continue
		}
		unite, image, etat := champs[0], champs[1], champs[2]
		switch etat {
		case "true":
			maj = append(maj, "• "+image+" ("+unite+")")
		case "failed", "rolled back":
			pb = append(pb, "• "+image+" ("+unite+") : "+etat)
		}
	}
	if rc != 0 && len(pb) == 0 {
		for _, l := range strings.Split(sortie, "\n") {
			if strings.HasPrefix(l, "Error") && len(pb) < 2 {
				pb = append(pb, l)
			}
		}
	}
	if len(maj) > 0 {
		c.e.lancer(nil, "podman", "image", "prune", "-f")
	}
	if len(maj) > 0 || len(pb) > 0 {
		msg := "Conteneurs Podman"
		if len(maj) > 0 {
			msg += ", mis à jour :\n" + strings.Join(maj, "\n")
		}
		if len(pb) > 0 {
			msg += "\nProblème :\n" + strings.Join(pb, "\n")
		}
		c.notifier(msg)
	}
	// LiteLLM redémarré : attendre qu'il réponde avant de demander l'avis du LLM.
	if strings.Contains(strings.Join(maj, "\n"), "litellm") {
		for i := 0; i < 60; i++ {
			if c.llmVivant() {
				break
			}
			time.Sleep(5 * time.Second)
		}
	}
}

// llmVivant : la passerelle LLM répond sur /health/liveliness.
func (c *contexteUpdate) llmVivant() bool {
	cl := &http.Client{Timeout: 5 * time.Second}
	racine := strings.TrimSuffix(c.r.llmURL, "/v1")
	r, err := cl.Get(racine + "/health/liveliness")
	if err != nil {
		return false
	}
	r.Body.Close()
	return r.StatusCode == 200
}

// lireLignes lit les lignes d'un fichier.
func lireLignes(chemin string) ([]string, error) {
	f, err := os.Open(chemin)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var res []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		res = append(res, sc.Text())
	}
	return res, sc.Err()
}

// motifCommit : le commit d'une version AAAA.MM.JJ.HHMM-<commit>[-dirty].
var motifCommit = regexp.MustCompile(`-([0-9a-f]+)(-dirty)?$`)

// ageImageJours : âge d'une image d'après sa date AAAA.MM.JJ au début de sa version.
func ageImageJours(version string, maintenant time.Time) int {
	if len(version) < 10 {
		return 0
	}
	d, err := time.Parse("2006.01.02", version[:10])
	if err != nil {
		return 0
	}
	return int(maintenant.Sub(d).Seconds()) / 86400
}

// appelerLLM envoie la consigne au modèle local et renvoie le texte de sa réponse.
func (c *contexteUpdate) appelerLLM(prompt string) (string, error) {
	corps := map[string]any{
		"model":       c.r.modele,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"temperature": 0.2,
		"max_tokens":  6000,
		"extra_body":  map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": false}},
	}
	b, _ := json.Marshal(corps)
	req, err := http.NewRequest("POST", strings.TrimSuffix(c.r.llmURL, "/")+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.r.llmKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.r.llmKey)
	}
	rep, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer rep.Body.Close()
	if rep.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", rep.StatusCode)
	}
	var r struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(rep.Body).Decode(&r); err != nil {
		return "", err
	}
	if len(r.Choices) == 0 {
		return "", fmt.Errorf("réponse sans choix")
	}
	return r.Choices[0].Message.Content, nil
}

// formaterImportants : les paquets importants, une puce par ligne (12 au plus).
func formaterImportants(decision map[string]any) string {
	liste, _ := decision["paquets_importants"].([]any)
	var lignes []string
	for i, x := range liste {
		if i >= 12 {
			break
		}
		lignes = append(lignes, fmt.Sprintf("• %v", x))
	}
	return strings.Join(lignes, "\n")
}

// trierCles : clés triées (sorties déterministes).
func trierCles(m map[string]string) []string {
	var k []string
	for x := range m {
		k = append(k, x)
	}
	sort.Strings(k)
	return k
}

