# Membuat modul Vikunja 2.8.0 untuk NethServer 8

Tanggal pemeriksaan: **22 November 2026**. Status: **kandidat pengembangan**,
belum lulus penerimaan NS8. Target akun GitHub: `amulyawan`.
Repository dan image akun tersebut baru tersedia setelah fork/push/publish disetujui.

## 1. Pilihan: fork modul yang ada

Gunakan `geniusdynamics/ns8-vikunja` sebagai dasar. Jangan membuat aplikasi
Vikunja baru. Yang kita buat adalah **paket integrasi NS8** untuk menjalankan
image resmi Vikunja yang sudah menyertakan frontend dan backend aplikasi.
Frontend Vue dalam folder `ui/` adalah panel pengaturan NS8, bukan frontend
Vikunja. PostgreSQL tetap dikelola di dalam pod aplikasi NS8.

- **Fork** membuat salinan repository di akun GitHub Anda, dengan hubungan upstream.
- **Clone** mengunduh repository ke komputer untuk dikerjakan.
- **Commit** menyimpan checkpoint perubahan secara lokal.
- **Push** mengirim commit ke GitHub.
- **Build** menghasilkan image paket NS8, misalnya `ghcr.io/amulyawan/vikunja:vikunja-2.8.0`.
- **Publish** mengunggah image ke GitHub Container Registry (GHCR).
- **Install** membuat instance aplikasi di NS8 menggunakan image tersebut.

Alur: fork → perubahan lokal → validasi → push → build/publish kandidat →
instalasi staging → uji migrasi dan pemulihan → persetujuan produksi.

Nomor versi modul dan aplikasi merupakan dua hal berbeda. Untuk kandidat ini,
aplikasi adalah **2.8.0** dan tag image modul mengikuti branch **vikunja-2.8.0**.
Gunakan tag kandidat yang unik; hindari `latest` untuk pengujian upgrade.

## 2. Identitas, ruang lingkup, dan prasyarat

| Field | Nilai |
| --- | --- |
| product_name / product_version | ns8-vikunja candidate / aplikasi 2.8.0 |
| stage / blast_radius | development / satu instance staging terpisah |
| target_os | NS8 di Linux; Windows hanya workstation pengembangan |
| target_hosts_or_host_class | VM NS8 staging, tidak berbagi data produksi |
| source_artifact_or_repo | fork dari geniusdynamics/ns8-vikunja |
| affected_consumers | pengguna Vikunja, database, lampiran, SMTP, klien API/MCP |
| configuration_files | state/environment, database.env, vikunja-db.env, vikunja.env, smarthost.env |
| data_store_policy | PostgreSQL di volume postgres-data; lampiran di volume vikunja-app |
| secrets_policy | secret dibuat di NS8; jangan dimasukkan ke repository atau log publik |
| ports_and_firewall_policy | backend dipetakan ke loopback; akses melalui Traefik/TLS |
| backup_path | snapshot VM + tujuan backup NS8 yang dicatat secara privat |
| rollback_command | restore snapshot VM sebelum upgrade; verifikasi versi dan dataset lama |
| rollback_status | prosedur tersedia; drill pemulihan NS8 belum dilakukan |
| license_status | GPL/AGPL dan komponen utama diperiksa; lihat DEPENDENCIES.md |
| prerequisite_status | build Linux dan NS8 staging masih harus divalidasi |
| validation_evidence | VALIDATION.md dan laporan lokal, bukan asumsi kompatibilitas |
| owner_confirmation_required | push, publish image, perubahan NS8 dan produksi |

Selesaikan prasyarat berikut sebelum langkah yang bergantung padanya:

| Prasyarat | Alasan / validasi | Status dan rollback |
| --- | --- | --- |
| Git, GitHub CLI, akses akun | `git --version`, `gh auth status` | tersedia di workstation; tanpa perubahan autentikasi |
| Bash dan Python 3 | `python3 tools/validate.py` | tersedia melalui WSL; pemeriksaan tanpa NS8 |
| Linux Buildah/Podman | `buildah version`, `podman version`, build kandidat | belum tersedia di workstation; gunakan runner GitHub atau VM build |
| Node/Corepack/Yarn | Node builder sesuai build-images.sh; Yarn sesuai packageManager | paket dipin; cache dan hasil build bisa dibuat ulang |
| NS8 staging dan Traefik | periksa versi core, node ID, DNS, TLS, kapasitas disk | belum diperiksa; NS8 core minimal 3.0.0 sesuai label modul |
| Backup target dan snapshot VM | uji restore ke VM/instance terpisah | wajib sebelum migrasi; sumber rollback utama |

Jangan mengunggah `.local`, file `.env`, dump database, kredensial, hostname
internal, alamat jaringan privat, screenshot berisi data nyata, atau log mentah.
Semua nama `example.org` dalam panduan ini adalah contoh sintetis.

## 3. Persiapan Windows

Source lokal yang disiapkan dalam pekerjaan ini sudah berasal dari upstream,
dengan branch `vikunja-2.8.0`. Anda tidak perlu melakukan clone kedua kali.
Periksa melalui PowerShell di folder repository:

```powershell
git status -sb
git log -1 --oneline
git diff main...HEAD --stat
wsl -d Ubuntu-24.04 -- bash -lc 'command -v python3; command -v bash'
```

Di terminal WSL, masuk ke folder repository yang sama, lalu:

```bash
python3 tools/validate.py
```

File shell harus memakai LF. `.gitattributes` mengatur hal ini untuk checkout
berikutnya. Jangan menjalankan container NS8 sebagai Windows Service.
Tidak perlu Docker Desktop: jalur utama menggunakan GitHub Actions + Buildah.

## 4. Fork dan unggah source — setelah persetujuan pemilik

Source kandidat harus sudah diuji, dipindai dari data privat, dan di-commit lokal.
Untuk menghindari perubahan remote clone saat fork, buat fork tanpa clone:

```powershell
gh repo fork geniusdynamics/ns8-vikunja --clone=false
git remote -v
git remote add owner https://github.com/amulyawan/ns8-vikunja.git
git push -u owner vikunja-2.8.0
```

Perintah `remote add owner` hanya dijalankan jika nama tersebut belum ada.
Jangan mengganti remote lain atau menggunakan force push. Fork repository
publik umumnya tetap publik; hanya source yang telah disanitasi boleh dikirim.
Fork/push di atas belum dijalankan dalam pekerjaan persiapan ini.

## 5. Apa yang diubah

1. `build-images.sh`: image resmi `docker.io/vikunja/vikunja:2.8.0`,
   namespace GHCR akun pemilik, instalasi Yarn `--immutable`, shell strict mode.
2. Konfigurasi baru memakai `VIKUNJA_SERVICE_SECRET`. Hook upgrade menyalin
   nilai JWT secret lama ke nama baru, mempertahankan password database dan
   host database kustom. Salinan pemulihan lokal dibuat sebelum perubahan.
3. Host PostgreSQL bawaan memakai loopback pod. Major PostgreSQL tetap **15**.
4. Volume lampiran menggunakan `:Z,U` untuk SELinux dan UID container 1000.
   `:U` menyesuaikan kepemilikan isi volume; uji dengan lampiran lama di staging.
5. Backup menyertakan konfigurasi database, secret, konfigurasi aplikasi, dump,
   serta volume lampiran. Backup yang dibuat oleh modul lama belum tentu lengkap.
6. Penemuan SMTP menghasilkan variabel `VIKUNJA_MAILER_*` yang benar dan
   dibaca ulang oleh container setiap start. Uji email sungguhan tetap diperlukan.
7. Validasi source otomatis tersedia. Publish image dibuat manual agar
   source push tidak langsung menjadi publikasi image kandidat.

MCP sudah menjadi bagian Vikunja 2.8.0 pada `/api/v2/mcp`; tidak perlu container
MCP tambahan atau membuka port kedua. Token dibuat melalui menu Settings → MCP.

## 6. Build dan publish dengan GitHub Actions

Sesudah fork/push disetujui:

1. Buka fork di GitHub → **Actions**. Aktifkan workflow jika GitHub memintanya.
2. Pastikan workflow **Validate source** pada branch kandidat berhasil.
3. Pilih **Publish images** → **Run workflow** → branch `vikunja-2.8.0`.
4. Tunggu validasi source, build, scan Trivy, lalu push. Scan HIGH/CRITICAL
   yang gagal akan menghalangi publikasi; perbaiki penyebabnya. Periksa detail error,
   jangan menganggap workflow hijau bila ada job yang dilewati.
5. Catat SHA commit, URL run, tag image, digest paket, dan versi aplikasi.
6. Buka Packages akun/repository → paket **vikunja**. Tentukan akses package
   yang disetujui. Image publik memudahkan pull NS8; package privat memerlukan
   autentikasi registry di NS8 yang harus direncanakan terpisah.
7. Pengujian NS8 dilakukan di staging mengikuti bagian 8–10. Workflow reusable
   upstream tidak digunakan dalam kandidat ini karena repository workflow belum
   menyediakan lisensi yang dapat diverifikasi. Workflow publikasi ditulis
   mandiri; pengujian NS8/QEMU otomatis belum tersedia.

Alternatif CLI, setelah otorisasi publikasi:

```powershell
gh workflow run publish-images.yml --repo amulyawan/ns8-vikunja --ref vikunja-2.8.0
gh run list --repo amulyawan/ns8-vikunja --limit 10
```

Hasil yang diharapkan: `ghcr.io/amulyawan/vikunja:vikunja-2.8.0`.
Modul NS8 ini adalah image paket berbasis scratch; pemeriksaan keamanan image
paket saja tidak memeriksa aplikasi. Scan juga image Vikunja dan PostgreSQL.
Periksa biaya/kuota Actions dan package akun sebelum penggunaan runner.

## 7. Alternatif build manual di Linux

Jalankan pada **VM build terpisah**. Perintah instalasi paket di bawah adalah
langkah persiapan, bukan perintah untuk mengubah host NS8 produksi.

Ubuntu 24.04 (workstation/VM build):

```bash
sudo apt update
sudo apt install git python3 buildah podman uidmap
buildah version
podman version
```

Rocky Linux 9 (VM build):

```bash
sudo dnf install git python3 buildah podman shadow-utils
getenforce
buildah version
podman version
```

Gunakan versi paket repositori distro tepercaya, catat versi aktual. Pastikan
user build mempunyai rentang subordinate UID/GID; jangan menonaktifkan SELinux
atau mengubah firewall untuk mengatasi kegagalan build.

Di kedua distro, dalam repository:

```bash
python3 tools/validate.py
REPOBASE=ghcr.io/amulyawan IMAGETAG=vikunja-2.8.0 bash build-images.sh
buildah inspect ghcr.io/amulyawan/vikunja
```

Periksa label `org.nethserver.images`, `org.nethserver.min-core`, port demand,
dan rootless. Jangan berasumsi label saja membuktikan runtime sehat.
Untuk publish manual yang sudah disetujui, gunakan login interaktif tanpa token
di command line atau di repository:

```bash
buildah login ghcr.io
buildah push ghcr.io/amulyawan/vikunja docker://ghcr.io/amulyawan/vikunja:vikunja-2.8.0
buildah logout ghcr.io
```

Jangan memasang runtime build ini pada server produksi hanya untuk build.
Rollback workstation: kembali ke checkpoint Git; hapus hanya image/container
build yang tepat setelah identitasnya diperiksa, tanpa `prune` global.

## 8. Instalasi baru pada NS8 staging

Tahap ini dilakukan setelah publikasi kandidat berhasil dan target NS8 disetujui.
Periksa distro dan versi core NS8, Traefik, DNS, sertifikat, node ID, disk,
dan backend backup. Gunakan distro yang didukung rilis NS8 aktual.
Untuk Ubuntu periksa firewall yang aktif; untuk Rocky periksa firewalld dan
SELinux. Jangan membuka PostgreSQL ke jaringan atau menonaktifkan SELinux.

Pada shell root **node staging NS8**, ganti `1` dengan node ID yang benar:

```bash
add-module ghcr.io/amulyawan/vikunja:vikunja-2.8.0 1
```

Catat `module_id` dari output. Contoh berikut mengasumsikan `vikunja1`;
jangan memakai contoh ini bila itu adalah nama instance produksi.
Siapkan DNS dan sertifikat tepercaya untuk hostname staging Anda.

```bash
api-cli run configure-module --agent module/vikunja1 --data - <<'JSON'
{"host":"vikunja.example.org","http2https":true,"lets_encrypt":false}
JSON
api-cli run get-configuration --agent module/vikunja1
runagent -m vikunja1 systemctl --user status vikunja.service vikunja-app.service postgresql-app.service
runagent -m vikunja1 podman ps
```

`lets_encrypt:false` tidak menyediakan sertifikat tepercaya secara otomatis.
Gunakan sertifikat NS8 yang sesuai, atau Let's Encrypt bila DNS dan jangkauan
validasinya memenuhi syarat. Jangan menonaktifkan verifikasi TLS pada klien.

Dari workstation dengan kepercayaan CA yang benar:

```bash
python3 tools/smoke.py https://vikunja.example.org
```

Atau uji backend secara lokal pada node NS8 dengan menyalin hanya script
`tools/smoke.py` ke lokasi staging yang disetujui dan memakai TCP_PORT instance.
Uji backend loopback tidak membuktikan DNS/TLS/Traefik bekerja.

## 9. Penerimaan staging dan MCP

Isi hasil setiap gate dalam `VALIDATION.md` dengan bukti tanggal/versi/digest.

1. Buat dua pengguna sintetis. Uji login, project, task, komentar, berbagi dan
   pembatasan hak akses pengguna kedua.
2. Upload/download lampiran sintetis; simpan checksum untuk dibandingkan.
3. Restart service instance, lalu pastikan data dan lampiran tetap ada.
4. Uji SMTP dan perubahan smarthost dengan tujuan email uji yang disetujui.
5. Settings → MCP → buat token **Read only** berumur pendek. Simpan privat.
6. Hubungkan klien ke `https://vikunja.example.org/api/v2/mcp` menggunakan
   instruksi yang dihasilkan Vikunja. Uji initialize, tools/list dan membaca
   task. Pastikan token hanya dapat mengakses data yang diizinkan.
7. Pastikan panggilan anonim ditolak. Jangan gunakan token pemilik berakses
   penuh untuk integrasi rutin. Cabut token uji setelah selesai.
8. Uji backup NS8 dan restore ke instance lain. Bandingkan pengguna, project,
   task, lampiran/checksum, secret dan konfigurasi. Jangan membagikan bukti mentah.

Token MCP adalah kredensial, bukan bagian source modul. Keberhasilan health
check atau penolakan anonim belum membuktikan MCP terautentikasi bekerja.

## 10. Upgrade dari modul 1.1.0

Jika belum ada instalasi lama, lakukan instalasi baru lebih dahulu. Jangan
menambahkan kompleksitas migrasi yang belum diperlukan.

Jika sudah ada instance lama:

1. Catat image/digest lama, versi database, konfigurasi, volume dan konsumen
   secara privat. Ambil backup NS8 **dan snapshot VM konsisten** sebelum upgrade.
2. Backup modul lama dapat tidak menyertakan `vikunja-db.env`; simpan salinan
   konfigurasi/secret itu secara privat di luar repository. Jangan mengandalkan
   backup lama yang belum pernah berhasil direstore.
3. Bangun dataset sintetis pada NS8 staging dengan modul 1.1.0. Untuk data nyata,
   penggunaan salinan produksi memerlukan persetujuan dan kontrol privasi tersendiri.
4. Uji upgrade staging ke kandidat; periksa log migrasi dan seluruh gate bagian 9.
   Migrasi database dijalankan oleh aplikasi saat startup. Jangan sekaligus
   mengganti major PostgreSQL. Jangan mengklaim lompatan 1.1.0 → 2.8.0 didukung
   hanya karena aplikasi bisa start dengan database kosong.
5. Perintah berikut hanya untuk **instance staging yang telah diverifikasi**:

```bash
api-cli run update-module --data '{"module_url":"ghcr.io/amulyawan/vikunja:vikunja-2.8.0","instances":["vikunja1"]}'
```

6. Periksa konfigurasi, versi API, akses pengguna, file, SMTP, token, dan backup.
   Integrasi lama yang memakai API v1 perlu pengujian/migrasi ke API v2.
7. Uji rollback pada staging sebelum menjadwalkan produksi. Rencana downtime
   dan persetujuan produksi adalah langkah terpisah.

**Rollback setelah migrasi database:** hentikan kandidat pada target yang tepat,
restore snapshot VM sebelum upgrade atau pulihkan backup lengkap beserta image
lama pada VM terpisah. Verifikasi versi lama dan dataset/checksum. Jangan hanya
menurunkan tag image pada database yang telah dimigrasi. Snapshot lama kehilangan
perubahan setelah snapshot; tetapkan batas waktu pemulihan secara privat.
Salinan `vikunja-db.env.pre-2.8` hanya memulihkan konfigurasi, bukan database.

## 11. Pemeliharaan rilis berikutnya

Periksa rilis stabil resmi, release notes, perubahan konfigurasi/API dan
image manifest. Buat branch baru dari kandidat terakhir yang tervalidasi,
perbarui versi, refresh catatan digest, lalu ulangi build, scan, staging,
upgrade dan restore. Publikasikan tag modul yang unik; jangan menimpa tag
yang sudah diterima produksi. Patch yang matang dapat diajukan kembali
ke upstream sebagai pull request setelah pemilik menyetujuinya.

## Referensi

- [Source modul awal](https://github.com/geniusdynamics/ns8-vikunja)
- [Vikunja 2.8.0](https://github.com/go-vikunja/vikunja/releases/tag/v2.8.0)
- [NS8: pembuatan modul](https://nethserver.github.io/ns8-core/modules/new_module/)
- [NS8: image dan label](https://nethserver.github.io/ns8-core/modules/images/)
- [NS8: manajemen modul](https://nethserver.github.io/ns8-core/modules/)
- [Vikunja MCP](https://vikunja.io/help/mcp/)
- [Vikunja API v2](https://vikunja.io/docs/api-v2/)
