# Instalasi uji Vikunja pada NS8 dan pembaruan melalui web UI

Dokumen terpisah dari panduan pembuatan modul. Target kandidat: Vikunja 2.8.0,
PostgreSQL 19.0, modul `2.8.0-rc.2`. Instalasi baru dan update kandidat pada NS8
aktual lulus pada 22 November 2026. Nilai token asli dan informasi infrastruktur
pemilik berada dalam dokumentasi lokal privat, pada bagian token step-by-step.

## 1. Pemeriksaan dan rollback

Periksa core NS8 minimal 3.0.0, node ID, ruang disk, SELinux, status layanan
lama, daftar aplikasi dan repository. Simpan baseline secara privat sebelum
menambah aplikasi atau repository. Buat instance uji baru dengan data sintetis.
Rollback hanya menghapus instance uji dan repository yang dibuat dalam pekerjaan
ini; jangan mengubah aplikasi email/directory yang sudah digunakan.

## 2. Token GitHub Packages

Pada akun pemilik, buka GitHub Settings → Developer settings → Personal access
tokens (classic) → Generate new token (classic).

- Nama: `NS8 Vikunja read-only images`.
- Scope: **read:packages saja**.
- Expiration: **No expiration**, sesuai keputusan pemilik.
- Simpan nilai satu kali tersebut dalam dokumentasi lokal privat yang dilindungi
  ACL, bukan source Git atau URL repository.

Dokumentasi lokal privat memuat **nilai token asli pada bagian token dalam
step-by-step instalasi**, account, scope, masa berlaku, lokasi auth dan prosedur
rotasi. Lokasi sebenarnya hanya dibagikan kepada pemilik melalui chat/laporan lokal.

Pada NS8 3.25.1, pengelola dan agent memakai `REGISTRY_AUTH_FILE` yang menunjuk ke
`/etc/nethserver/registry.json`. Tambahkan hanya namespace
`ghcr.io/amulyawan/ns8-vikunja-build` sambil mempertahankan entry registry lainnya
dan mode bawaan NS8. File ini adalah konfigurasi registry tingkat node yang
digunakan agent NS8. Backup sebelumnya harus disimpan root-only.

Salinan `/root/.config/containers/auth.json` mode `0600` dapat dipakai untuk
perintah Podman eksplisit, tetapi login di situ saja tidak menyediakan auth bagi
lifecycle aplikasi NS8. Begitu juga file auth pribadi agent tidak dipakai jika
`REGISTRY_AUTH_FILE` menunjuk ke konfigurasi platform. Periksa environment aktual.
Auth di `/run/user/...` saja tidak cukup untuk pembaruan setelah reboot.
Jangan menaruh token pada command line atau history shell.

Contoh pada shell root NS8 (Rocky Linux yang sudah menjalankan NS8), setelah
memastikan file native tersebut memang dipakai oleh versi core Anda:

```bash
authfile=/etc/nethserver/registry.json
authmode=$(stat -c %a "$authfile")
registry_backup="/root/ns8-registry-before-$(date +%Y%m%d-%H%M%S).json"
install -m 600 "$authfile" "$registry_backup"
read -r -s -p 'GitHub token read:packages: ' registry_token
printf '\n'
printf '%s' "$registry_token" | podman login --authfile "$authfile" \
  --username amulyawan --password-stdin ghcr.io/amulyawan/ns8-vikunja-build
unset registry_token
chmod "$authmode" "$authfile"
podman pull ghcr.io/amulyawan/ns8-vikunja-build/vikunja:2.8.0-rc.2
```

Token diketik pada prompt tersembunyi, bukan ditempel sebagai perintah. Jika
login gagal, periksa error dan pulihkan file backup sebelum melanjutkan; jangan
menghapus entry registry lain. File native yang ditemukan mode `0644`; auth
bersifat tingkat node dan dapat dibaca pengguna lokal. Pertahankan kontrak core
ini agar agent rootless dapat melakukan pull. Nilai token dokumentasi lokal
hanya dapat dibaca pemilik dan SYSTEM melalui ACL Windows.

Untuk menjalankan perintah dari Windows, gunakan SSH ke node NS8 yang sudah
terdaftar pada konfigurasi privat pemilik. Build Linux dari Windows memakai
WSL atau runner GitHub; jangan menjalankan Bash dengan path `D:\...` tanpa
konversi. Contoh validasi source dari WSL:

```bash
cd /path/to/ns8-vikunja
python3 tools/validate.py
```


Uji pull image sebagai pengelola dan sebagai agent aplikasi. Token tidak bisa
digunakan untuk push, mengubah repository source, atau menghapus package.

Rotasi: buat token baru dengan scope sama, perbarui file auth terkait, uji pull,
lalu cabut token lama. Pencabutan dilakukan pada halaman Tokens (classic) GitHub.
Token No expiration tetap aktif sampai dicabut oleh pemilik.

## 3. Repository Software Center

URL repository NS8 adalah **direktori yang berisi repodata.json**, bukan URL Git
clone. Untuk modul ini:

```text
https://raw.githubusercontent.com/amulyawan/ns8-vikunja/vikunja-2.8.0/catalog/
```

Katalog hanya memuat aplikasi Vikunja. Image tetap privat walaupun metadata
katalog dan source tersedia publik. Tidak ada token dalam URL atau metadata.

Tambahkan melalui Settings → Software repositories atau menu Software Center:

- Nama: `zz-amulyawan-vikunja`.
- URL: seperti di atas.
- Enabled: aktif.
- Testing releases: aktif untuk kandidat `-rc`.

Nama repository ini sengaja diprioritaskan untuk Vikunja agar versi lama dari
repository lain tidak dipilih. Tidak ada aplikasi lain dalam katalog ini.
Refresh repository, lalu pastikan aplikasi berasal dari namespace pemilik.

## 4. Instalasi dan konfigurasi

Gunakan Software Center untuk memilih Vikunja dari repository pemilik, atau
API cluster `add-module` dengan image `vikunja` dan node ID yang sudah diperiksa.
Sediakan auth platform untuk pull runtime database privat sebelum create-module
menjalankan langkah download. Catat module_id aktual dari hasil API.

Logo katalog memakai `logo: "logo.png"`; NS8 sendiri menambahkan direktori ID
aplikasi. Menulis `vikunja/logo.png` akan menggandakan direktori dan membuat
gambar gagal dimuat dengan fallback teks “undefined logo”. Pada pemeriksaan
akhir, gambar Vikunja benar-benar dimuat 256×256. Teks `alt` “undefined logo”
masih dihasilkan core NS8 untuk seluruh baris Applications, termasuk aplikasi
lain yang gambarnya normal; perbaikan katalog mengatasi gambar yang rusak.
Perubahan core untuk teks aksesibilitas itu belum dilakukan.

Konfigurasikan hostname uji yang tidak dipakai aplikasi lain melalui panel
Vikunja. Gunakan sertifikat tepercaya untuk pengujian browser. Backend aplikasi
hanya dipetakan ke loopback, sedangkan Traefik menangani akses HTTPS.

## 5. Pengujian penerimaan

Periksa unit dan container berjalan, versi API 2.8.0 dan PostgreSQL 19.0,
volume `postgres19-data` di `/var/lib/postgresql`, serta volume lampiran.
Uji pengguna/project/task/lampiran sintetis, restart, API/MCP, dan backup–restore.
Hindari pengiriman email nyata saat menjalankan data uji.

Simpan hasil dan log secara privat. Dokumen publik hanya menyatakan hasil yang
benar-benar terbukti. Health check bukan pengganti pengujian izin atau restore.

## 6. Pembaruan berikutnya dari web UI

Pemelihara membangun image dengan tag semver baru, menguji/scan, lalu memperbarui
`catalog/repodata.json` dari label image yang telah diperiksa. NS8 memerlukan
metadata versi baru agar dapat menawarkan update; push source saja bukan rilis.

Di NS8: Software Center → Reload repositories → pilih Vikunja → Update.
Auth persisten pengelola dan agent memungkinkan pull image privat. Ambil backup
aplikasi sebelum update dan verifikasi data setelah update selesai.

Upgrade volume PostgreSQL 15 langsung ke 19 diblokir. Migrasikan secara logis
ke instance terpisah dan verifikasi dahulu; jangan mengubah versi server database
dengan memasang direktori data lama pada container major baru.


## 7. Backup–restore khusus aplikasi uji

1. Di Backup and restore, pilih destination backup lokal yang sudah siap. Buat
   job baru yang hanya mencakup instance Vikunja uji. Jangan mengedit atau
   menjalankan job produksi untuk menguji satu aplikasi.
2. Untuk uji manual, aktifkan sementara job tersebut dengan jadwal jauh di masa
   depan, lalu Run backup. Pada core 3.25.1, menjalankan job **disabled** mengembalikan
   sukses tanpa membuat snapshot; verifikasi snapshot dan ukuran, bukan exit code
   saja. Setelah run selesai, kembalikan job uji menjadi disabled.
3. Catat repository/path/snapshot dari hasil Read snapshots. Simpan bukti secara
   privat. Backup modul meliputi dump PostgreSQL, konfigurasi database, secret
   aplikasi, konfigurasi Vikunja dan volume lampiran.
4. Pilih Restore dari snapshot tersebut ke **instance baru**, dengan `replace:false`.
   Pastikan hostname di backup bebas. Untuk uji salinan pada node yang sama,
   ubah hostname instance sumber uji dahulu ke hostname sintetis lain, sambil
   mempertahankan allowlist. Source tetap dipertahankan.
5. Untuk staging yang tidak boleh terekspos atau mengirim email, siapkan route
   destination dengan `ip_allowlist:["127.0.0.1/32"]` sebelum restore. Konfigurasi
   route oleh modul mempertahankan allowlist yang sudah ada. Pasang override
   `VIKUNJA_MAILER_ENABLED=false` hanya pada instance uji sebelum layanan pertama
   berjalan. Override systemd lokal ini bukan bagian backup modul dan harus
   disiapkan kembali pada destination uji. Helper langkah aktual disimpan lokal
   privat bersama dokumentasi pemilik.
6. Setelah restore, verifikasi frontend/health/versi, login dengan akun sintetis,
   project/task, dan unduh lampiran; checksum harus sama. Bandingkan konfigurasi
   database dan service secret secara programatis tanpa mencetak nilainya.
7. Setelah lulus, hapus hanya salinan restore sintetis yang identitasnya sudah
   diverifikasi. Pertahankan instance sumber dan snapshot, lalu pulihkan hostname
   staging sumber. Jangan menjalankan prune global atau menghapus job produksi.

Untuk migrasi dari modul lama 1.1.0/PostgreSQL 15, prosedur ini harus diperluas
menjadi migrasi logis yang diuji terpisah. Kandidat memblokir pemasangan volume
major lama langsung pada PostgreSQL 19. Uji restore kandidat baru ini tidak
membuktikan migrasi dataset lama.

## 8. Hasil aktual 22 November 2026

| Pemeriksaan | Hasil |
| --- | --- |
| Platform | NS8 3.25.1, Rocky 9.8, Podman 5.8.2, SELinux Enforcing |
| Instalasi normal baru | Lulus, melalui lifecycle add-module/configure |
| Versi runtime | Vikunja 2.8.0 dan PostgreSQL 19.0 |
| Login/project/task | Lulus dengan data sintetis |
| Lampiran | Upload, download dan SHA-256 sama |
| Restart | Task dan lampiran tetap tersedia |
| Izin | Pengguna kedua ditolak membaca task/lampiran pengguna pertama |
| MCP | Anonymous ditolak; initialize/tools/list/tasks_read lulus; write token baca-saja ditolak |
| Backup–restore native | Lulus ke instance terpisah, login/data/file/DB credential/secret terpelihara |
| Update web UI | RC1→RC2 melalui Review and update, data dan konfigurasi tetap utuh |
| Logo Applications | Lulus, gambar 256×256 dengan URL katalog yang benar |
| Aplikasi lama | 17 versi dan digest modul lama tidak berubah |
| Cleanup | Salinan restore dihapus setelah verifikasi; job backup uji disabled |

Build/scan/publish RC2: [run 44963956861](https://github.com/amulyawan/ns8-vikunja-build/actions/runs/44963956861).
Ketiga image lulus gate HIGH/CRITICAL; kedua package terverifikasi privat.
Katalog menawarkan RC2 dan browser menunjukkan versi tersebut setelah update.

Instance yang ditinggalkan tetap **staging**: route dibatasi loopback dan email
keluar dinonaktifkan. Belum diuji: migrasi dataset 1.1.0, komentar/berbagi,
TLS/DNS domain tim, pengiriman email nyata, serta reboot node. Jangan menyebut
hasil ini sebagai penerimaan produksi lengkap.

## 9. Rollback dan pemeriksaan berikutnya

Untuk membatalkan pemasangan uji baru, identifikasi instance dari output instalasi,
ambil backup bila ingin mempertahankan data, kemudian hapus hanya instance uji
dan repository `zz-amulyawan-vikunja`. Pulihkan hanya perubahan entry auth yang
terkait dari backup privat; jangan menimpa entry baru milik pekerjaan lain.
Backup snapshot dan dokumen token tidak perlu dihapus untuk rollback aplikasi.

Untuk kegagalan update, restore snapshot sebelum update ke instance terpisah
beserta image kandidat sebelumnya dan verifikasi data dahulu. Jangan downgrade
major PostgreSQL pada volume yang sudah dimigrasikan. Drill RC1 restore berhasil
pada pekerjaan ini; rollback ke aplikasi lama 1.1.0 belum diuji.

Langkah berikut sebelum pemakaian tim: tetapkan domain/TLS dan akses yang sesuai,
uji SMTP nyata ke alamat uji pemilik, tetapkan jadwal backup operasional, lalu
jalankan penerimaan penggunaan tim. Pengujian ini tidak membuka route staging
ke publik atau mengubah kebijakan email seluruh cluster.
