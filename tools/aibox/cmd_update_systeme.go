package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// systeme : comparaison de l'image installée aux dépôts, décision, build,
// vérification et installation (même ordre que le script).
func (c *contexteUpdate) systeme() int {
	boot := strings.TrimSpace(firstLine(c.e.combine(nil, "blkid", "-L", "AIBOXBOOT")))
	if boot == "" {
		c.journaliser("Pas de partition AIBOXBOOT (démarrage sur clé ?) : rien à faire.")
		return 0
	}
	work, err := os.MkdirTemp("", "aibox-auto-update")
	if err != nil {
		c.journaliser("aibox update : " + err.Error())
		return 1
	}
	defer c.nettoyer(work)

	// 1. Copie du dépôt, remise à origin/<ref> par aibox build.
	commitSortie, err := c.e.lancer(nil, "sudo", "aibox", "build", "--fetch-only", "--ref", c.r.buildRef)
	if err != nil {
		return c.echec("mise à jour de la copie du dépôt impossible (réseau ? GitHub ?) : voir " + c.journal + ".")
	}
	commit := strings.TrimSpace(commitSortie)
	src := filepath.Join(c.r.buildDir, "src")
	c.journaliser(fmt.Sprintf("Dépôt : origin/%s à %s", c.r.buildRef, commit))

	// 2. Paquets de l'image par défaut sur AIBOXBOOT (lus en root, partition en 0077).
	esp := filepath.Join(work, "esp")
	os.Mkdir(esp, 0o755)
	if _, err := c.e.lancer(nil, "sudo", "mount", "-o", "ro,umask=0077", boot, esp); err != nil {
		return c.echec("montage de AIBOXBOOT impossible.")
	}
	image, nomImage, lireErr := c.lireImageInstallee(esp)
	c.e.lancer(nil, "sudo", "umount", esp)
	if lireErr != nil {
		return c.echec(lireErr.Error())
	}
	if c.o.pkglist != "" {
		b, err := os.ReadFile(c.o.pkglist)
		if err != nil {
			fmt.Fprintf(c.erreurs, "%s introuvable.\n", c.o.pkglist)
			return 2
		}
		image = string(b)
		c.journaliser("Référence : " + c.o.pkglist + " (--pkglist)")
	}
	versionImage := strings.TrimPrefix(nomImage, "aibox-")
	c.journaliser("Image par défaut : " + versionImage)

	// 3. Versions disponibles dans les dépôts du build (base pacman jetable).
	db := filepath.Join(work, "db")
	os.Mkdir(db, 0o755)
	conf := filepath.Join(src, "archiso", "pacman.conf")
	if _, err := c.e.lancer(nil, "sudo", "pacman", "--config", conf, "--dbpath", db, "-Sy"); err != nil {
		return c.echec("synchronisation des dépôts impossible (réseau ?).")
	}
	listes, _ := c.e.lancer(nil, "pacman", "--config", conf, "--dbpath", db, "-Sl")
	dispo := depotsParPaquet(listes)
	maj := paquetsAMettreAJour(image, dispo, c.vercmp)
	count := len(maj)
	c.journaliser(fmt.Sprintf("Paquets à mettre à jour : %d", count))

	// 4. Nouveaux commits : le commit de l'image est à la fin de sa version.
	commitImage := ""
	if m := motifCommit.FindStringSubmatch(versionImage); m != nil {
		commitImage = m[1]
	}
	commits := ""
	if commitImage != "" && commitImage != commit {
		sortieLog, err := c.e.lancer(nil, "sudo", "git", "-C", src, "log", "--oneline", commitImage+"..HEAD")
		switch {
		case err != nil:
			commits = fmt.Sprintf("(commit de l'image %s inconnu du dépôt)", commitImage)
		case strings.TrimSpace(sortieLog) == "":
			commits = fmt.Sprintf("(commit de l'image %s différent de %s)", commitImage, commit)
		default:
			commits = strings.TrimSpace(sortieLog)
		}
		c.journaliser("Nouveaux commits depuis " + commitImage + " :\n" + commits)
	}
	age := ageImageJours(versionImage, c.maintenant())

	// 5. Décision : --force, nouveaux commits, rien de neuf, sinon l'avis du LLM.
	var decision map[string]any
	switch {
	case c.o.force:
		decision = map[string]any{"mettre_a_jour": true, "raison": "lancé avec --force", "paquets_importants": []any{}}
	case commits != "":
		decision = map[string]any{"mettre_a_jour": true, "raison": "nouveaux commits sur GitHub :\n" + premieresLignes(commits, 12), "paquets_importants": []any{}}
	case count == 0:
		c.journaliser("Rien de nouveau dans les dépôts.")
		return 0
	default:
		prompt := promptMaj(versionImage, age, count, premieresLignes(texteMaj(maj), 300))
		contenu, err := c.appelerLLM(prompt)
		if err != nil {
			return c.echec(fmt.Sprintf("le LLM ne répond pas (%s, modèle %s) : pas de décision, pas de build.", c.r.llmURL, c.r.modele))
		}
		d, ok := extraireDecision(contenu)
		if !ok {
			return c.echec("réponse du LLM illisible (pas de JSON avec mettre_a_jour).")
		}
		decision = d
	}
	enregistrerDecision(c.etat, decision, versionImage, count, c.maintenant(), texteMaj(maj))
	update, _ := decision["mettre_a_jour"].(bool)
	raison, _ := decision["raison"].(string)
	c.journaliser(fmt.Sprintf("Décision : %t — %s", update, raison))
	if !update {
		return 0
	}
	if c.o.dryRun {
		c.journaliser("--dry-run : pas de build.")
		return 0
	}

	// 6. Build (aibox build, en root), sortie dans build.log.
	c.journaliser("Construction de l'image…")
	journalBuild := filepath.Join(c.etat, "build.log")
	fb, err := os.Create(journalBuild)
	if err != nil {
		return c.echec(err.Error())
	}
	rc := c.e.flux(nil, fb, "sudo", "aibox", "build", "--ref", c.r.buildRef)
	fb.Close()
	if rc != 0 {
		txt, _ := os.ReadFile(journalBuild)
		return c.echec("le build a échoué : voir " + journalBuild + "\n" + lignesErreur(string(txt), 3))
	}
	txtBuild, _ := os.ReadFile(journalBuild)
	versionNeuve := derniereVersion(string(txtBuild))
	iso, err := filepath.EvalSymlinks(filepath.Join(c.r.buildDir, "out", "aibox-latest.iso"))
	if err != nil || filepath.Base(iso) != "aibox-"+versionNeuve+"-x86_64.iso" {
		return c.echec(fmt.Sprintf("ISO construite introuvable (%s, version « %s »).", iso, versionNeuve))
	}
	if strings.HasSuffix(versionNeuve, "-dirty") {
		return c.echec(fmt.Sprintf("image -dirty (%s) : pas installée.", versionNeuve))
	}

	// 7. Vérification de l'image avant toute installation.
	problemes, err := c.verifierImage(work, iso, versionNeuve, src, maj)
	if err != nil {
		return c.echec(err.Error())
	}
	if len(problemes) > 0 {
		return c.echec("image " + versionNeuve + " refusée à la vérification :\n" + premieresLignes(strings.Join(problemes, "\n")+"\n", 10))
	}
	c.journaliser("Image " + versionNeuve + " vérifiée.")

	if c.o.noInstall {
		c.journaliser("--no-install : image construite et vérifiée, pas installée (" + iso + ").")
		return 0
	}

	// 8. Installation : l'ancienne version reste au menu de démarrage.
	journalInstall := filepath.Join(c.etat, "installation.log")
	fi, err := os.Create(journalInstall)
	if err != nil {
		return c.echec(err.Error())
	}
	rc = c.e.flux(nil, fi, "sudo", "aibox", "boot", "--iso", iso)
	fi.Close()
	if rc != 0 {
		return c.echec("aibox boot a échoué : voir " + journalInstall)
	}
	msg := fmt.Sprintf("Nouvelle image installée : %s (remplace %s, %d paquets mis à jour).\nRaison : %s\n", versionNeuve, versionImage, count, raison)
	if imp := formaterImportants(decision); imp != "" {
		msg += imp + "\n"
	}
	msg += "Elle démarrera au prochain allumage ; l'ancienne reste au menu. Arrêt complet puis rallumage conseillé (pas de redémarrage à chaud)."
	c.notifier(msg)
	return 0
}

// lireImageInstallee : nom (aibox-…) de l'image par défaut, lue dans loader.conf,
// et sa liste de paquets (« nom version »).
func (c *contexteUpdate) lireImageInstallee(esp string) (string, string, error) {
	loader, _ := c.e.lancer(nil, "sudo", "cat", filepath.Join(esp, "loader", "loader.conf"))
	m := regexp.MustCompile(`(?m)^default (.*)\.conf$`).FindStringSubmatch(loader)
	if m == nil {
		return "", "", fmt.Errorf("liste des paquets de l'image par défaut introuvable sur AIBOXBOOT.")
	}
	nom := m[1]
	liste, err := c.e.lancer(nil, "sudo", "cat", filepath.Join(esp, nom, "pkglist.x86_64.txt"))
	if err != nil || strings.TrimSpace(liste) == "" {
		return "", "", fmt.Errorf("liste des paquets de %s introuvable sur AIBOXBOOT.", nom)
	}
	return liste, nom, nil
}

// verifierImage : version, droits (profiledef.sh) et versions des paquets, dans
// l'ISO montée. Renvoie la liste des problèmes (vide si l'image est bonne).
func (c *contexteUpdate) verifierImage(work, iso, version, src string, maj []paquetMaj) ([]string, error) {
	isoMnt := filepath.Join(work, "iso")
	rootMnt := filepath.Join(work, "root")
	os.Mkdir(isoMnt, 0o755)
	os.Mkdir(rootMnt, 0o755)
	if _, err := c.e.lancer(nil, "sudo", "mount", "-o", "loop,ro", iso, isoMnt); err != nil {
		return nil, fmt.Errorf("montage de l'ISO %s impossible.", version)
	}
	defer func() {
		c.e.lancer(nil, "sudo", "umount", rootMnt)
		c.e.lancer(nil, "sudo", "umount", isoMnt)
	}()
	if _, err := c.e.lancer(nil, "sudo", "mount", "-t", "erofs", "-o", "loop,ro", filepath.Join(isoMnt, "aibox", "x86_64", "airootfs.erofs"), rootMnt); err != nil {
		return nil, fmt.Errorf("montage de l'ISO %s impossible.", version)
	}

	var problemes []string
	if b, err := os.ReadFile(filepath.Join(isoMnt, "aibox", "version")); err != nil || strings.TrimSpace(string(b)) != version {
		problemes = append(problemes, "version de l'image différente de "+version)
	}

	pd, _ := os.ReadFile(filepath.Join(src, "archiso", "profiledef.sh"))
	droits := profilDroits(string(pd))
	chemins := make([]string, 0, len(droits))
	for ch := range droits {
		chemins = append(chemins, ch)
	}
	sort.Strings(chemins)
	for _, ch := range chemins {
		attendu := droits[ch]
		actuel, err := c.e.lancer(nil, "sudo", "stat", "-c", "%a", rootMnt+ch)
		if err != nil {
			if _, err := os.Stat(filepath.Join(src, "archiso", "airootfs", ch)); err == nil {
				problemes = append(problemes, "absent : "+ch)
			}
			continue
		}
		if a := strings.TrimSpace(actuel); a != attendu {
			problemes = append(problemes, fmt.Sprintf("droits de %s : %s au lieu de %s", ch, a, attendu))
		}
	}

	pkgl, _ := os.ReadFile(filepath.Join(isoMnt, "aibox", "pkglist.x86_64.txt"))
	for _, p := range maj {
		got := ""
		for _, l := range strings.Split(string(pkgl), "\n") {
			if champs := strings.Fields(l); len(champs) >= 2 && champs[0] == p.nom {
				got = champs[1]
				break
			}
		}
		if got == "" || c.vercmp(got, p.nouvelle) < 0 {
			if got == "" {
				got = "absent"
			}
			problemes = append(problemes, fmt.Sprintf("paquet %s : %s au lieu de %s", p.nom, got, p.nouvelle))
		}
	}
	time.Sleep(time.Second)
	return problemes, nil
}

// vercmp compare deux versions de paquets comme pacman (négatif, nul ou positif).
func (c *contexteUpdate) vercmp(a, b string) int {
	out, err := c.e.lancer(nil, "vercmp", a, b)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}

// nettoyer supprime le dossier de travail (les montages et la base pacman
// appartiennent à root : suppression avec sudo, seulement sous ce dossier).
func (c *contexteUpdate) nettoyer(work string) {
	if strings.HasPrefix(work, filepath.Join(os.TempDir(), "aibox-auto-update")) {
		c.e.lancer(nil, "sudo", "rm", "-rf", "--", work)
	}
}

// enregistrerDecision écrit derniere-decision.json et derniers-paquets.txt.
func enregistrerDecision(etat string, decision map[string]any, versionImage string, count int, maintenant time.Time, paquets string) {
	sortie := map[string]any{}
	for k, v := range decision {
		sortie[k] = v
	}
	sortie["image"] = versionImage
	sortie["paquets"] = count
	sortie["date"] = maintenant.UTC().Format("2006-01-02T15:04:05Z")
	if b, err := json.MarshalIndent(sortie, "", "  "); err == nil {
		os.WriteFile(filepath.Join(etat, "derniere-decision.json"), append(b, '\n'), 0o644)
	}
	os.WriteFile(filepath.Join(etat, "derniers-paquets.txt"), []byte(paquets), 0o644)
}

// lignesErreur : les premières lignes qui parlent d'une erreur dans un journal.
func lignesErreur(txt string, n int) string {
	re := regexp.MustCompile(`(?i)error|erreur|cannot|place|secrets|authorized_keys`)
	var res []string
	for _, l := range strings.Split(txt, "\n") {
		if re.MatchString(l) && len(res) < n {
			res = append(res, l)
		}
	}
	return strings.Join(res, "\n")
}

