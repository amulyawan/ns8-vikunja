# Instalasi uji Vikunja pada NS8 dan pembaruan melalui web UI

Dokumen terpisah dari panduan pembuatan modul. Target kandidat: Vikunja 2.8.0,
PostgreSQL 19.0, modul `2.8.0-rc.1`. Pengujian pada NS8 aktual sedang berjalan;
hasil terverifikasi ditambahkan setelah selesai. Nilai token dan informasi
infrastruktur pemilik hanya berada dalam dokumentasi lokal privat.

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
gambar gagal dimuat dengan fallback teks “undefined logo”.

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
