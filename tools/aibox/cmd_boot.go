package main

import (
	"bufio"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
)

const usageBoot = `Usage : sudo aibox boot [--iso FICHIER.iso] [--keep 2]
  Installe une image aibox sur AIBOXBOOT (partition de démarrage), sans toucher aux
  données. L'image est celle qui a démarré (clé Ventoy), ou --iso (obligatoire quand
  on a démarré depuis le disque). Seules les --keep versions les plus récentes restent
  (2 : la nouvelle et la précédente) ; celle qui tourne n'est jamais supprimée.`

// optionsBoot : options de aibox boot.
type optionsBoot struct {
	iso  string
	keep int
}

// imageBoot : ce qui est lu dans l'ISO à installer.
type imageBoot struct {
	src, basedir, version string
	kernel, initrd, opts  string
}

var (
	motifLinux    = regexp.MustCompile(`(?m)^\s*linux\s+(.*)$`)
	motifInitrd   = regexp.MustCompile(`(?m)^\s*initrd\s+(\S+)`)
	motifBasedir  = regexp.MustCompile(`(?:^| )archisobasedir=(\S+)`)
	motifUuidProc = regexp.MustCompile(`(?:^| )archisosearchuuid=(\S+)`)
	motifBasedirP = regexp.MustCompile(`(?:^| )(?:archisobasedir|archisosearchuuid|archisolabel|archisodevice)=\S+`)
	motifCopyRAM  = regexp.MustCompile(`(?:^| )copytoram=\S+`)
	motifNomImage = regexp.MustCompile(`^aibox-[0-9][0-9A-Za-z._-]*$`)
)

// cmdBoot : « aibox boot », installe une image sur la partition de démarrage.
func cmdBoot(args []string, sortie, erreurs io.Writer) int {
	o := optionsBoot{keep: 2}
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--iso" && i+1 < len(args):
			o.iso = args[i+1]
			i++
		case args[i] == "--keep" && i+1 < len(args):
			n := 0
			if _, err := fmt.Sscanf(args[i+1], "%d", &n); err != nil || n < 1 {
				fmt.Fprintln(erreurs, usageBoot)
				return 1
			}
			o.keep = n
			i++
		default:
			fmt.Fprintln(erreurs, usageBoot)
			return 1
		}
	}
	if os.Geteuid() != 0 {
		fmt.Fprintf(erreurs, "À lancer en root : sudo aibox boot %s\n", strings.Join(args, " "))
		return 1
	}
	return installerBoot(envReel(), o, sortie, erreurs)
}

// installerBoot monte l'ISO et la partition, puis installe l'image.
func installerBoot(e env, o optionsBoot, sortie, erreurs io.Writer) int {
	bootPart := strings.TrimSpace(firstLine(e.combine(nil, "blkid", "-L", "AIBOXBOOT")))
	if bootPart == "" {
		fmt.Fprintln(erreurs, "Pas de partition AIBOXBOOT : préparer le disque avec aibox setup-disk.")
		return 1
	}
	work, err := os.MkdirTemp("", "aibox-boot")
	if err != nil {
		fmt.Fprintf(erreurs, "aibox boot : %v\n", err)
		return 1
	}
	defer os.RemoveAll(work)
	src := filepath.Join(work, "src")
	esp := filepath.Join(work, "esp")
	os.Mkdir(src, 0o755)
	os.Mkdir(esp, 0o755)
	defer e.lancer(nil, "umount", "-q", esp)
	defer e.lancer(nil, "umount", "-q", src)

	if o.iso != "" {
		if info, err := os.Stat(o.iso); err != nil || info.IsDir() {
			fmt.Fprintf(erreurs, "%s introuvable.\n", o.iso)
			return 1
		}
		if _, err := e.lancer(nil, "mount", "-o", "loop,ro", o.iso, src); err != nil {
			fmt.Fprintf(erreurs, "aibox boot : montage de l'ISO impossible : %v\n", err)
			return 1
		}
	} else {
		cmdline, _ := e.lire("proc/cmdline")
		m := motifUuidProc.FindStringSubmatch(cmdline)
		if m == nil || !e.existe("dev/disk/by-uuid/"+m[1]) {
			fmt.Fprintln(erreurs, "Démarré depuis le disque (ou image introuvable) : donner --iso FICHIER.iso.")
			return 1
		}
		if _, err := e.lancer(nil, "mount", "-o", "ro", "/dev/disk/by-uuid/"+m[1], src); err != nil {
			fmt.Fprintf(erreurs, "aibox boot : montage impossible : %v\n", err)
			return 1
		}
	}

	img, err := lireImage(src)
	if err != nil {
		fmt.Fprintln(erreurs, err)
		return 1
	}
	if _, err := e.lancer(nil, "mount", "-o", "umask=0077", bootPart, esp); err != nil {
		fmt.Fprintf(erreurs, "aibox boot : montage de AIBOXBOOT impossible : %v\n", err)
		return 1
	}
	uuid, _ := e.lancer(nil, "blkid", "-s", "UUID", "-o", "value", bootPart)
	cmdline, _ := e.lire("proc/cmdline")
	running := ""
	if m := motifBasedir.FindStringSubmatch(cmdline); m != nil {
		running = m[1]
	}
	return installerDansEsp(e, esp, img, strings.TrimSpace(uuid), running, o.keep, bootPart, sortie, erreurs)
}

// lireImage lit grub.cfg (ligne linux, initrd, archisobasedir) et la version.
func lireImage(src string) (imageBoot, error) {
	grub := filepath.Join(src, "boot", "grub", "grub.cfg")
	b, err := os.ReadFile(grub)
	if err != nil {
		return imageBoot{}, fmt.Errorf("Ce n'est pas une ISO aibox (pas de boot/grub/grub.cfg).")
	}
	txt := string(b)
	lin := motifLinux.FindStringSubmatch(txt)
	ini := motifInitrd.FindStringSubmatch(txt)
	if lin == nil || ini == nil {
		return imageBoot{}, fmt.Errorf("Ce n'est pas une ISO aibox (entrée linux ou initrd absente).")
	}
	champs := strings.Fields(lin[1])
	img := imageBoot{src: src, kernel: champs[0], initrd: ini[1], opts: strings.TrimSpace(strings.TrimPrefix(lin[1], champs[0]))}
	bd := motifBasedir.FindStringSubmatch(img.opts)
	if bd == nil {
		return imageBoot{}, fmt.Errorf("Ce n'est pas une ISO aibox (archisobasedir absent).")
	}
	img.basedir = bd[1]
	v, err := os.ReadFile(filepath.Join(src, img.basedir, "version"))
	if err != nil {
		return imageBoot{}, fmt.Errorf("version de l'image illisible : %v", err)
	}
	img.version = strings.TrimSpace(string(v))
	for _, f := range []string{img.kernel, img.initrd, "/" + img.basedir + "/x86_64/airootfs.erofs"} {
		if _, err := os.Stat(filepath.Join(src, f)); err != nil {
			return imageBoot{}, fmt.Errorf("Fichier manquant dans l'ISO : %s", f)
		}
	}
	return img, nil
}

// installerDansEsp copie l'image, la vérifie, écrit les entrées du menu et ne
// garde que les dernières versions. Ne touche jamais à la version qui tourne.
func installerDansEsp(e env, esp string, img imageBoot, bootUUID, running string, keep int, bootPart string, sortie, erreurs io.Writer) int {
	name := "aibox-" + img.version
	if !motifNomImage.MatchString(name) {
		fmt.Fprintf(erreurs, "Version d'image inattendue : %q.\n", img.version)
		return 1
	}
	// Place libre : l'image entière, plus une marge.
	besoin := mio(tailleArbre(filepath.Join(img.src, img.basedir))) + 64
	libre := espaceLibreEsp(esp)
	if info, err := os.Stat(filepath.Join(esp, name)); err == nil && info.IsDir() {
		libre += mio(tailleArbre(filepath.Join(esp, name)))
	}
	if libre < besoin {
		fmt.Fprintf(erreurs, "Pas assez de place sur AIBOXBOOT (%d Mo libres, %d Mo nécessaires) : réduire --keep.\n", libre, besoin)
		return 1
	}

	fmt.Fprintf(sortie, "Copie de l'image aibox %s sur %s…\n", img.version, bootPart)
	nouveau := filepath.Join(esp, name+".new")
	os.RemoveAll(nouveau)
	if err := copierArbre(filepath.Join(img.src, img.basedir), nouveau); err != nil {
		os.RemoveAll(nouveau)
		fmt.Fprintf(erreurs, "Copie impossible : %v\n", err)
		return 1
	}
	if err := verifierSha512(filepath.Join(nouveau, "x86_64")); err != nil {
		os.RemoveAll(nouveau)
		fmt.Fprintf(erreurs, "Copie de l'image corrompue (somme SHA-512 fausse) : %v\n", err)
		return 1
	}
	if err := comparerArbres(filepath.Join(img.src, img.basedir, "boot"), filepath.Join(nouveau, "boot")); err != nil {
		os.RemoveAll(nouveau)
		fmt.Fprintf(erreurs, "Copie du noyau ou de l'initramfs corrompue : %v\n", err)
		return 1
	}
	if _, err := os.Stat(filepath.Join(esp, name)); err == nil {
		if name == running {
			fmt.Fprintf(sortie, "Remplace la version en cours (%s) : elle reste en RAM jusqu'au redémarrage.\n", name)
		}
		os.RemoveAll(filepath.Join(esp, name))
	}
	if err := os.Rename(nouveau, filepath.Join(esp, name)); err != nil {
		fmt.Fprintf(erreurs, "aibox boot : %v\n", err)
		return 1
	}

	// Entrées du menu : la partition est désignée par son UUID.
	// Options de l'ISO sans les désignations de support ; l'entrée « sans copie en
	// RAM » retire copytoram puis le remet à n (même logique que le script).
	optsNettes := strings.TrimSpace(motifBasedirP.ReplaceAllString(img.opts, ""))
	sansCopie := strings.TrimSpace(motifCopyRAM.ReplaceAllString(optsNettes, ""))
	entrees := filepath.Join(esp, "loader", "entries")
	ecrireEntree(entrees, name, "aibox "+img.version, img, name, bootUUID, optsNettes, "")
	ecrireEntree(entrees, name+"-noram", "aibox "+img.version+" (sans copie en RAM, dépannage)", img, name, bootUUID, sansCopie, " copytoram=n")
	loader := fmt.Sprintf("default %s.conf\ntimeout 3\neditor  no\n", name)
	if err := os.WriteFile(filepath.Join(esp, "loader", "loader.conf"), []byte(loader), 0o644); err != nil {
		fmt.Fprintf(erreurs, "aibox boot : %v\n", err)
		return 1
	}

	// Ne garder que les keep versions les plus récentes (jamais celle qui tourne).
	versions := versionsAibox(esp)
	for _, old := range versions[:maxInt(0, len(versions)-keep)] {
		if old == running || old == name {
			continue
		}
		os.RemoveAll(filepath.Join(esp, old))
		os.Remove(filepath.Join(entrees, old+".conf"))
		os.Remove(filepath.Join(entrees, old+"-noram.conf"))
		fmt.Fprintf(sortie, "Ancienne version retirée : %s\n", old)
	}

	fmt.Fprintln(sortie)
	fmt.Fprintln(sortie, "Versions sur AIBOXBOOT :")
	for _, v := range versionsAibox(esp) {
		fmt.Fprintf(sortie, "  %s\n", v)
	}
	fmt.Fprintf(sortie, "Par défaut au prochain démarrage : aibox %s (les autres restent dans le menu).\n", img.version)
	fmt.Fprintln(sortie, "Redémarrer pour l'utiliser : sudo systemctl reboot")
	return 0
}

// ecrireEntree écrit une entrée systemd-boot (même format qu'aibox setup-disk).
func ecrireEntree(dossier, id, titre string, img imageBoot, name, bootUUID, opts, suffixe string) {
	linux := "/" + name + strings.TrimPrefix(img.kernel, "/"+img.basedir)
	initrd := "/" + name + strings.TrimPrefix(img.initrd, "/"+img.basedir)
	contenu := fmt.Sprintf("title    %s\nversion  %s\nsort-key aibox\nlinux    %s\ninitrd   %s\noptions  archisobasedir=%s archisodevice=UUID=%s %s%s\n",
		titre, img.version, linux, initrd, name, bootUUID, strings.TrimSpace(opts), suffixe)
	os.WriteFile(filepath.Join(dossier, id+".conf"), []byte(contenu), 0o644)
}

// versionsAibox : les dossiers aibox-N de l'ESP, du plus ancien au plus récent.
func versionsAibox(esp string) []string {
	entrees, _ := os.ReadDir(esp)
	var res []string
	for _, d := range entrees {
		if d.IsDir() && motifNomImage.MatchString(d.Name()) && !strings.HasSuffix(d.Name(), ".new") {
			res = append(res, d.Name())
		}
	}
	sort.Slice(res, func(i, j int) bool { return versionAvant(res[i], res[j]) })
	return res
}

// verifierSha512 : chaque ligne de airootfs.sha512 doit correspondre au fichier.
func verifierSha512(dossier string) error {
	b, err := os.ReadFile(filepath.Join(dossier, "airootfs.sha512"))
	if err != nil {
		return err
	}
	vus := 0
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		champs := strings.Fields(sc.Text())
		if len(champs) < 2 {
			continue
		}
		contenu, err := os.ReadFile(filepath.Join(dossier, strings.TrimPrefix(champs[1], "*")))
		if err != nil {
			return err
		}
		somme := sha512.Sum512(contenu)
		if hex.EncodeToString(somme[:]) != champs[0] {
			return fmt.Errorf("%s", champs[1])
		}
		vus++
	}
	if vus == 0 {
		return fmt.Errorf("liste vide")
	}
	return nil
}

// comparerArbres : mêmes fichiers, mêmes contenus (équivalent de diff -rq).
func comparerArbres(a, b string) error {
	err := filepath.WalkDir(a, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(a, p)
		x, err1 := os.ReadFile(p)
		y, err2 := os.ReadFile(filepath.Join(b, rel))
		if err1 != nil || err2 != nil || string(x) != string(y) {
			return fmt.Errorf("%s diffère", rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return filepath.WalkDir(b, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(b, p)
		if _, err := os.Stat(filepath.Join(a, rel)); err != nil {
			return fmt.Errorf("%s en trop", rel)
		}
		return nil
	})
}

// copierArbre copie un dossier (fichiers et sous-dossiers ; pas de liens symboliques : FAT).
func copierArbre(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		cible := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(cible, 0o755)
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("lien symbolique refusé : %s", rel)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(cible, b, 0o644)
	})
}

// tailleArbre : taille totale des fichiers d'un dossier, en octets.
func tailleArbre(dossier string) uint64 {
	var total uint64
	filepath.WalkDir(dossier, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += uint64(info.Size())
			}
		}
		return nil
	})
	return total
}

// espaceLibreEsp : Mo libres sur la partition de démarrage (df -m).
func espaceLibreEsp(esp string) int64 {
	var s syscall.Statfs_t
	if err := syscall.Statfs(esp, &s); err != nil {
		return 0
	}
	return int64(s.Bavail*uint64(s.Bsize)) / (1 << 20)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
