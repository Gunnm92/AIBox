package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const usageNas = `Usage : aibox nas list | mount | add //hôte/partage /mnt/X | remove /mnt/X
  aibox nas list                            partages configurés et leur état
  sudo aibox nas add //hôte/partage /mnt/X  ajoute un partage (identifiants demandés une fois)
  sudo aibox nas remove /mnt/X              retire un partage ajouté
  sudo aibox nas mount                      (re)monte tout (utilisé au démarrage)
Partages de l'image : /etc/aibox/nas-mounts (identifiants /etc/aibox/nas.cred), fournis au build.
Partages ajoutés : /var/lib/aibox/state/nas/mounts, identifiants dans /var/lib/aibox/state/nas/<hôte>.cred.`

// optionsCifs : options de montage reprises du script (mêmes valeurs).
const optionsCifs = "vers=3.0,uid=1000,gid=1000,rsize=1048576,wsize=1048576,cache=loose,actimeo=30,file_mode=0664,dir_mode=0775,nounix,noserverino,mfsymlinks,_netdev"

// partageNas : une ligne SOURCE CIBLE [IDENTIFIANTS].
type partageNas struct {
	src, tgt, cred string
}

// cmdNas : « aibox nas », partages réseau SMB montés à la demande.
func cmdNas(args []string, sortie, erreurs io.Writer) int {
	e := envReel()
	entree := bufio.NewReader(os.Stdin)
	if len(args) == 0 {
		fmt.Fprintln(erreurs, usageNas)
		return 1
	}
	if args[0] != "list" && os.Geteuid() != 0 {
		fmt.Fprintf(erreurs, "À lancer en root : sudo aibox nas %s\n", strings.Join(args, " "))
		return 1
	}
	switch args[0] {
	case "list":
		nasList(e, sortie)
		return 0
	case "mount":
		nasMonterTout(e, sortie)
		return 0
	case "add":
		if len(args) != 3 {
			fmt.Fprintln(erreurs, "usage : aibox nas add //hôte/partage /mnt/X")
			return 1
		}
		return nasAjouter(e, args[1], args[2], entree, sortie, erreurs)
	case "remove":
		if len(args) != 2 {
			fmt.Fprintln(erreurs, "usage : aibox nas remove /mnt/X")
			return 1
		}
		return nasRetirer(e, args[1], sortie, erreurs)
	}
	fmt.Fprintln(erreurs, usageNas)
	return 1
}

// cheminsNas : fichiers de configuration et dossier d'état, sous la racine de e.
func cheminsNas(e env) (confImage, credImage, etat, confUtilisateur string) {
	etat = e.chemin("var/lib/aibox/state/nas")
	return e.chemin("etc/aibox/nas-mounts"), e.chemin("etc/aibox/nas.cred"), etat, filepath.Join(etat, "mounts")
}

// lignesNas : les partages de l'utilisateur d'abord, puis ceux de l'image qui
// ne sont pas remplacés par un ajout (même cible).
func lignesNas(e env) []partageNas {
	confImage, credImage, etat, confUtilisateur := cheminsNas(e)
	var res []partageNas
	vus := map[string]bool{}
	for _, l := range lignesFichier(confUtilisateur) {
		vus[l.tgt] = true
		res = append(res, l)
	}
	for _, l := range lignesFichier(confImage) {
		if vus[l.tgt] {
			continue
		}
		if l.cred == "" {
			l.cred = credImage
		}
		res = append(res, l)
	}
	for i := range res {
		if res[i].cred == "" {
			res[i].cred = filepath.Join(etat, hoteDe(res[i].src)+".cred")
		}
	}
	return res
}

// lignesFichier : lit « SOURCE CIBLE [IDENTIFIANTS] », sans les lignes vides ni commentées.
func lignesFichier(chemin string) []partageNas {
	b, err := os.ReadFile(chemin)
	if err != nil {
		return nil
	}
	var res []partageNas
	for _, l := range strings.Split(string(b), "\n") {
		champs := strings.Fields(l)
		if len(champs) < 2 || strings.HasPrefix(champs[0], "#") {
			continue
		}
		p := partageNas{src: champs[0], tgt: champs[1]}
		if len(champs) > 2 {
			p.cred = champs[2]
		}
		res = append(res, p)
	}
	return res
}

// hoteDe : l'hôte d'une source //hôte/partage.
func hoteDe(src string) string {
	champs := strings.Split(src, "/")
	if len(champs) > 2 {
		return champs[2]
	}
	return ""
}

// etatNas : « connecté », « prêt » (monté au premier accès) ou « non monté ».
func etatNas(e env, cible string) string {
	for _, m := range e.montages() {
		if m.point != cible {
			continue
		}
		switch m.typ {
		case "cifs":
			return "connecté"
		case "autofs":
			return "prêt (monté au premier accès)"
		}
	}
	return "non monté"
}

// nasList affiche les partages et leur état.
func nasList(e env, sortie io.Writer) {
	for _, p := range lignesNas(e) {
		fmt.Fprintf(sortie, "%-28s %-18s %s\n", p.src, p.tgt, etatNas(e, p.tgt))
	}
}

// monterNas monte un partage à la demande, s'il ne l'est pas déjà.
func monterNas(e env, p partageNas, sortie io.Writer) {
	for _, m := range e.montages() {
		if m.point == p.tgt {
			return
		}
	}
	if _, err := os.Stat(p.cred); err != nil {
		fmt.Fprintf(sortie, "Identifiants absents pour %s (%s).\n", p.src, p.cred)
		return
	}
	os.MkdirAll(e.chemin(p.tgt), 0o755)
	_, err := e.lancer(nil, "systemd-mount", "--no-block", "--automount=yes", "--collect", "-t", "cifs",
		"-o", "credentials="+p.cred+","+optionsCifs, p.src, p.tgt)
	if err == nil {
		fmt.Fprintf(sortie, "Monté à la demande : %s → %s\n", p.src, p.tgt)
	}
}

// nasMonterTout remonte tous les partages (démarrage).
func nasMonterTout(e env, sortie io.Writer) {
	for _, p := range lignesNas(e) {
		monterNas(e, p, sortie)
	}
}

// nasAjouter ajoute un partage à la configuration de l'utilisateur, demande les
// identifiants une seule fois, puis monte le partage.
func nasAjouter(e env, src, tgt string, entree *bufio.Reader, sortie, erreurs io.Writer) int {
	if !strings.HasPrefix(src, "//") || strings.Count(src, "/") < 3 {
		fmt.Fprintln(erreurs, "Source attendue : //hôte/partage")
		return 1
	}
	if strings.ContainsAny(src+tgt, " \t") {
		fmt.Fprintln(erreurs, "Pas d'espace dans la source ni dans la cible.")
		return 1
	}
	if !montageExiste(e, "var/lib/aibox/state") {
		fmt.Fprintln(erreurs, "Pas de disque de données (mode éphémère) : impossible de garder ce réglage.")
		return 1
	}
	_, _, etat, confUtilisateur := cheminsNas(e)
	hote := hoteDe(src)
	cred := filepath.Join(etat, hote+".cred")
	if err := os.MkdirAll(etat, 0o700); err != nil {
		fmt.Fprintf(erreurs, "aibox nas : %v\n", err)
		return 1
	}
	os.Chmod(etat, 0o700)
	if info, err := os.Stat(cred); err != nil || info.Size() == 0 {
		fmt.Fprintf(sortie, "Utilisateur sur %s : ", hote)
		utilisateur, _ := entree.ReadString('\n')
		fmt.Fprint(sortie, "Mot de passe : ")
		motDePasse, _ := lireLigneTerminal(entree)
		fmt.Fprintln(sortie)
		contenu := fmt.Sprintf("username=%s\npassword=%s\n", strings.TrimSpace(utilisateur), motDePasse)
		if err := os.WriteFile(cred, []byte(contenu), 0o600); err != nil {
			fmt.Fprintf(erreurs, "aibox nas : %v\n", err)
			return 1
		}
	}

	// Une ligne par cible : l'ajout remplace l'éventuel partage précédent.
	existant, _ := os.ReadFile(confUtilisateur)
	var garde []string
	for _, l := range strings.Split(string(existant), "\n") {
		if champs := strings.Fields(l); len(champs) >= 2 && champs[1] == tgt {
			continue
		}
		if strings.TrimSpace(l) != "" {
			garde = append(garde, l)
		}
	}
	garde = append(garde, src+" "+tgt)
	if err := ecrireAtomique(confUtilisateur, strings.Join(garde, "\n")+"\n"); err != nil {
		fmt.Fprintf(erreurs, "aibox nas : %v\n", err)
		return 1
	}
	monterNas(e, partageNas{src: src, tgt: tgt, cred: cred}, sortie)
	return 0
}

// nasRetirer retire un partage ajouté (pas ceux de l'image) et le démonte.
func nasRetirer(e env, tgt string, sortie, erreurs io.Writer) int {
	_, _, _, confUtilisateur := cheminsNas(e)
	existant, err := os.ReadFile(confUtilisateur)
	if err != nil {
		fmt.Fprintf(erreurs, "%s n'est pas un partage ajouté (ceux de l'image se changent dans keys/nas-mounts, au build).\n", tgt)
		return 1
	}
	var garde []string
	trouve := false
	for _, l := range strings.Split(string(existant), "\n") {
		champs := strings.Fields(l)
		if len(champs) >= 2 && champs[1] == tgt {
			trouve = true
			continue
		}
		if strings.TrimSpace(l) != "" {
			garde = append(garde, l)
		}
	}
	if !trouve {
		fmt.Fprintf(erreurs, "%s n'est pas un partage ajouté (ceux de l'image se changent dans keys/nas-mounts, au build).\n", tgt)
		return 1
	}
	if err := ecrireAtomique(confUtilisateur, strings.Join(garde, "\n")+"\n"); err != nil {
		fmt.Fprintf(erreurs, "aibox nas : %v\n", err)
		return 1
	}
	e.lancer(nil, "systemd-mount", "--umount", tgt)
	fmt.Fprintf(sortie, "Partage retiré : %s\n", tgt)
	return 0
}

// montageExiste : vrai si un montage existe à ce chemin système.
func montageExiste(e env, rel string) bool {
	_, ok := e.trouverMontage("/" + strings.TrimPrefix(rel, "/"))
	return ok
}

// ecrireAtomique écrit dans un fichier temporaire, puis le renomme : pas de
// configuration à moitié écrite.
func ecrireAtomique(chemin, contenu string) error {
	tmp := chemin + ".new"
	if err := os.WriteFile(tmp, []byte(contenu), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, chemin)
}

// lireLigneTerminal coupe l'écho du terminal (termios) le temps de lire une ligne.
func lireLigneTerminal(entree *bufio.Reader) (string, error) {
	var fd uintptr = 0
	var t syscall.Termios
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&t))); err != 0 {
		// Pas un terminal (tests, redirection) : lecture simple.
		ligne, err := entree.ReadString('\n')
		return strings.TrimRight(ligne, "\r\n"), err
	}
	sans := t
	sans.Lflag &^= syscall.ECHO
	syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(&sans)))
	defer syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(&t)))
	ligne, err := entree.ReadString('\n')
	return strings.TrimRight(ligne, "\r\n"), err
}
