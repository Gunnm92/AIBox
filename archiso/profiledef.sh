#!/usr/bin/env bash
# shellcheck disable=SC2034
# Profil aibox : système live CachyOS chargé entièrement en RAM.

iso_name="aibox"
iso_label="AIBOX_$(date --date="@${SOURCE_DATE_EPOCH:-$(date +%s)}" +%Y%m)"
iso_publisher="aibox"
iso_application="aibox"
iso_version="$(date --date="@${SOURCE_DATE_EPOCH:-$(date +%s)}" +%Y.%m.%d)"
install_dir="aibox"
bootmodes=('uefi.grub')
pacman_conf="pacman.conf"

# EROFS lz4hc : l'image est bloquée en RAM (copytoram), la compression réduit
# cette part ; la décompression lz4 est négligeable.
airootfs_image_type="erofs"
airootfs_image_tool_options=('-zlz4hc,12' '-Eztailpacking')
bootstrap_tarball_compression=('zstd' '-c' '-T0' '-19')

# copytoram (y : image copiée en RAM, clé retirable ; n : dépannage) est donné
# par chaque entrée du menu (grub/grub.cfg), pas ici.
# cow_spacesize=1G : plafond des écritures dans le système.
kernel_params_x86_64="cow_spacesize=1G ttm.pages_limit=28835840 ttm.page_pool_size=4194304"

file_permissions=(
  ["/etc/shadow"]="0:0:400"
  ["/etc/sudoers.d"]="0:0:750"
  ["/etc/sudoers.d/10-wheel"]="0:0:440"
  ["/usr/local/bin/aibox-mem"]="0:0:755"
  ["/usr/local/bin/xfdesktop-settings"]="0:0:755"
  ["/usr/local/bin/aibox-keyboard"]="0:0:755"
  ["/usr/local/bin/aibox-tv-launcher"]="0:0:755"
  ["/usr/local/bin/aibox-diag"]="0:0:755"
  ["/usr/local/sbin/aibox-setup-disk"]="0:0:755"
  ["/usr/local/sbin/aibox-update-boot"]="0:0:755"
  ["/usr/local/lib/aibox/"]="0:0:755"
  ["/etc/aibox/novnc.htpasswd"]="1000:1000:600"
  ["/etc/aibox/wayvnc-rdp.conf"]="1000:1000:600"
  ["/etc/aibox/novnc-tls.key"]="1000:1000:600"
)
