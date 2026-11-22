# Akses jaringan dan login LDAP/AD Vikunja pada NS8

Kandidat modul `2.8.0-rc.3` menambahkan konfigurasi jaringan dan domain pengguna
pada panel Settings NS8. Image registry tetap privat; pilihan akses aplikasi
tidak mengubah visibilitas GHCR.

## 1. Mengatasi Forbidden dan memilih akses

Jika backend sehat tetapi browser menerima HTTP 403, periksa **Network access**.
Allowlist `127.0.0.1/32` hanya menerima koneksi localhost. Mengganti hostname
mempertahankan allowlist lama; hal tersebut tidak otomatis membuka akses.

- **Only selected IP addresses or networks**: masukkan IPv4, IPv6 atau CIDR
  klien, dipisahkan koma. Contoh dokumentasi: `192.0.2.0/24, 2001:db8::/64`.
- **All IP addresses**: semua alamat klien dapat menjangkau halaman HTTPS;
  pengguna tetap harus login ke Vikunja.

Hostname dapat berada pada DNS privat atau publik. Pastikan DNS, sertifikat,
firewall dan NAT sesuai. Pilihan ini mengatur route HTTP aplikasi, bukan membuka
port database atau backend Podman langsung ke jaringan. Tidak ada perubahan
terhadap route aplikasi NS8 lainnya.

## 2. Menghubungkan domain Samba AD atau OpenLDAP

1. Pastikan domain sudah tersedia pada **Domains and users** NS8.
2. Buka **Applications → instance Vikunja → Settings**.
3. Isi hostname, pertahankan HTTPS, lalu pilih domain pada **User domain**.
4. Pilih **LDAP email attribute** sesuai isi directory, sebagaimana bagian 3.
5. Pilih Network access dan tekan **Save**.
6. Jika panel lama masih tersimpan dalam browser, muat ulang halaman setelah
   update modul. Panel harus menampilkan User domain dan Network access.
7. Buka hostname HTTPS aplikasi dan login dengan username serta sandi domain.
   AD juga dapat dicari memakai UPN. Akun admin panel NS8 bukan otomatis akun AD.

Modul menggunakan `agent.ldapproxy` dan binding domain native NS8, melalui role
`cluster:accountconsumer`. Ldapproxy mengurus koneksi TLS ke backend directory.
Pod aplikasi mengakses listener loopback melalui gateway slirp4netns; LDAP
tidak dibuka ke Internet.

Kredensial pencarian directory berasal dari NS8 dan dihasilkan ulang pada start
dalam `ldap.env` mode 0600. Nilainya tidak ditampilkan pada panel atau Git.
Pergantian konfigurasi domain memicu penyegaran layanan aplikasi. Jika domain
tidak tersedia, helper gagal agar layanan tidak beralih diam-diam ke login lokal.

Saat domain dipilih, form login lokal dan pendaftaran publik dimatikan. Kandidat
RC4 juga menonaktifkan akun lokal aktif melalui CLI resmi Vikunja, karena flag
form saja tidak cukup untuk menolak kredensial lokal yang sudah ada. Identitas
dan status awal disimpan dalam `local-users-before-directory.json` mode privat
dan disertakan backup. Akun dan datanya tidak dihapus. Mengosongkan domain tidak
mengaktifkan kembali akun tersebut secara otomatis; administrator memeriksa
backup lalu memakai CLI `user change-status <id> --enable` bila diperlukan.
Group synchronization tidak otomatis aktif. LDAP bukan SSO browser Kerberos/OIDC.

## 3. Mailbox NS8, atribut mail, dan UPN berbeda

**Mailbox** adalah tempat menerima/menyimpan pesan di Mail NS8. Jika domain Mail
mengaktifkan **Add user addresses from user domain**, alamat `username@domain`
dapat dibentuk dari nama akun tanpa mengisi atribut LDAP `mail`.

**Atribut `mail`** adalah field email pada record pengguna directory. Field itu
opsional pada NS8. Vikunja memakai atribut email yang dipilih sebagai alamat
profil dan tujuan notifikasi; aplikasi membutuhkan nilai email yang valid.
Mailbox yang ada tidak membuktikan bahwa atribut ini terisi.

**UPN (`userPrincipalName`)** adalah identitas login AD. Bentuknya menyerupai
email, tetapi belum tentu merupakan alamat mailbox yang aktif. Memilih UPN
memakai nilai tersebut untuk seluruh domain, bukan fallback otomatis per akun.

Untuk notifikasi yang benar, gunakan `mail` berisi alamat aktual. Jika diperlukan,
administrator dapat mengisi hanya nilai `mail` yang kosong setelah mencocokkan
mailbox aktif dan alamat/alias yang berlaku. Buat backup dan minta persetujuan
untuk perubahan directory bersama; pertahankan nilai yang sudah terisi. Modul
Vikunja tidak melakukan pengisian atau perubahan pengguna AD secara otomatis.

UPN tersedia sebagai alternatif jika memang cocok dengan kebijakan identitas
instansi. Verifikasi alamat tujuan sebelum mengaktifkan email keluar. Jangan
menganggap domain AD sama dengan domain email.

## 4. Validasi dan pemulihan

Periksa health dan versi aplikasi, mode akses pada route aktual, bind dan
pencarian LDAP dari jaringan pod, serta penolakan akun lokal, akun AD yang tidak
ada, dan pendaftaran publik. Login AD yang berhasil diuji pemilik langsung pada
halaman HTTPS; jangan menaruh sandi di chat, Git atau log.

Network access, domain, dan atribut email disimpan pada environment NS8 dan
disertakan dalam alur restore. Kredensial bind sementara dihasilkan ulang dari
domain di node tujuan. Simpan backup sebelum mengganti domain atau scope akses.

Untuk rollback akses, kembalikan allowlist sebelumnya melalui Settings. Untuk
rollback pengisian `mail` yang telah disetujui, gunakan backup per akun, cocokkan
identitas dan nilai terkini, lalu kembalikan hanya field tersebut; jangan
memulihkan seluruh database AD atau menimpa perubahan admin lain.

Lihat [prosedur instalasi/backup/update](INSTALASI-UJI-NS8-ID.md) dan
[hasil validasi](VALIDATION.md).

Referensi: [LDAP Vikunja](https://vikunja.io/docs/ldap/),
[user domains NS8](https://nethserver.github.io/ns8-core/core/user_domains/),
[Mail NS8](https://docs.nethserver.org/docs/administrator-manual/applications/mail).

## 5. Akun pertama, sandi, dan hak admin

Vikunja tidak menyediakan username/password bawaan. Tanpa LDAP, administrator
membuat akun lokal lewat registrasi yang diizinkan atau CLI. Dengan LDAP,
pengguna memakai kredensial directory yang sudah ada; profil Vikunja dibuat
pada login LDAP pertama yang berhasil, atau lebih awal melalui provisioning
opsional di bawah. Tidak ada sandi aplikasi bersama.

Admin NS8 mengelola paket, konfigurasi dan backup. Admin Samba mengelola directory.
Keduanya tidak otomatis menjadi admin seluruh instance Vikunja. Akun pertama juga
tidak otomatis mendapat hak tersebut. Admin project/team adalah izin berbagi
yang berbeda dan tersedia pada penggunaan biasa.

Pada 2.8.0, panel admin seluruh instance adalah fitur Pro dan memerlukan lisensi
serta penunjukan akun secara eksplisit melalui CLI `user set-admin`. Paket ini
tidak menambahkan lisensi atau memintas pembatasan Pro. Pengelolaan server melalui
NS8 dan CLI dasar tetap tersedia. Lihat [ketentuan resmi](https://vikunja.io/docs/pro/).

## 6. Akun AD baru dan pencarian anggota tim

Pemeriksaan baca-saja pada source NS8 menunjukkan bahwa Email pada pembuatan
user adalah opsional dan kosong secara default. Samba hanya mengisi atribut
`mail` bila nilai Email diberikan. Mail NS8 tetap dapat menyediakan alamat dan
mailbox logis melalui opsi `addusers`; itu tidak mengisi `mail` AD.

Karena itu, isi **Email** dengan alamat aktual ketika membuat akun baru yang
akan memakai Vikunja LDAP. Pengisian data lama yang disetujui pemilik bukan
otomatisasi untuk akun berikutnya. Pemeriksaan ini tidak membuat akun uji AD.
Waktu persis pembuatan direktori fisik Maildir belum dibuktikan melalui uji
runtime akun baru; jangan menyamakan direktori fisik dengan mailbox logis.

Login LDAP bawaan bukan impor seluruh directory ke tabel pengguna Vikunja.
Tanpa provisioning opsional, pengguna harus berhasil login sekali sebelum dapat
dipilih sebagai anggota tim atau assignee. Cari username persis, pilih hasil
autocomplete, kemudian Add to Team.
Mengetik nama bebas tanpa memilih hasil akan memunculkan “Please select a user”.
Pencarian nama tampilan/email mengikuti pilihan discoverability pengguna atau
hubungan external team; menambah anggota tim tidak membutuhkan panel admin Pro.

Referensi: [pembuatan user NS8](https://docs.nethserver.org/docs/administrator-manual/installation/user_domains#create-users-and-groups),
[alur LDAP Vikunja](https://vikunja.io/docs/ldap/) dan
[API pencarian pengguna](https://try.vikunja.io/api/v1/docs).

## 7. Provisioning LDAP sebelum login pertama (kandidat RC5)

Fitur ini merupakan tambahan modul NS8, bukan perilaku bawaan Vikunja.
Image aplikasi web tetap Vikunja resmi 2.8.0. Companion memakai model pembuatan
user dan identitas LDAP dari source versi yang sama, termasuk pemeriksaan lisensi.

1. Buat backup NS8 untuk instance ini dan pastikan backup benar-benar menghasilkan
   snapshot. Sediakan kapasitas database untuk user dan Inbox yang akan dibuat.
2. Pasang kandidat yang sudah lulus build, test, dan scan. Pilih domain NS8 serta
   atribut email yang benar pada **Settings**. `mail` harus berisi email valid.
3. Untuk memeriksa rencana sebelum aktivasi, di shell node NS8 jalankan:

   ```bash
   runagent -m vikunja1 sync-ldap-profiles --dry-run
   ```

   Ganti `vikunja1` dengan instance aktual. Ini tidak membuat atau mengubah profil;
   inisialisasi CLI standar tetap bisa menjalankan migrasi/metadata internal.
   Hasil hanya berisi jumlah `would_create`, `would_update`, `existing`, atau
   akun yang dilewati. Tidak ada password atau daftar directory dalam output.
4. Centang **Provision LDAP users before their first login**, lalu Save.
   Modul menjalankan provisioning pertama dan timer setiap sekitar 15 menit.
   API setara adalah `ldap_sync_enabled: true` pada `configure-module`.
5. Periksa status dari node:

   ```bash
   runagent -m vikunja1 systemctl --user status vikunja-ldap-sync.timer
   runagent -m vikunja1 cat ldap-profile-sync-status.json
   ```

6. Di **Team Members**, cari nama/username, klik hasil autocomplete, kemudian
   **Add to Team**. Pengguna tidak perlu sudah pernah login. Login pengguna tetap
   memakai kredensial AD masing-masing.

Hanya akun yang terlihat pada domain NS8 dan aktif yang diproses. User tanpa
email valid atau identitas stabil dilewati; akun lokal dengan username sama tidak
diambil alih. Pencarian dibatasi 5.000 hasil agar scope tak sengaja terlalu luas
gagal dengan aman. GUID AD dipakai seperti login native agar tidak membuat
profil kedua ketika pengguna akhirnya login.

Profil baru memiliki Inbox pribadi, tanpa password lokal, tanpa hak admin dan
tanpa keanggotaan tim otomatis. Nama profil baru dapat dicari oleh pengguna
Vikunja yang login; ini adalah dampak privasi yang disetujui lewat checkbox.
Pilihan discoverability dan status disabled pada profil yang sudah ada dijaga.
Sinkronisasi hanya memperbarui nama/email dari AD. Tidak menulis ke AD, tidak
membuat mailbox, tidak menyinkronkan grup, dan tidak menghapus akun yang hilang
dari directory. Penonaktifan AD menolak login LDAP berikutnya, tetapi fitur ini
bukan pencabutan sesi aktif; gunakan kontrol sesi jika perlu penonaktifan segera.

Untuk berhenti, hapus centang dan Save. Profil yang sudah dibuat tetap tersedia.
Manifest ID yang dibuat disimpan privat dalam `ldap-profile-sync-created.json`
dan ikut backup; jangan unggah file tersebut. Jika perlu rollback sesudah dipakai,
utamakan menonaktifkan akun yang ditinjau melalui CLI native, bukan menghapus
data user secara massal. Restore penuh hanya dari snapshot yang sesuai dan
setelah meninjau perubahan pengguna sejak snapshot.

Companion menambah indeks unik parsial `ns8_ldap_identity_unique` pada identitas
LDAP untuk menjaga konsistensi dengan login pertama yang berbarengan. Versi
aplikasi harus tepat 2.8.0; upgrade versi Vikunja harus menguji ulang companion.
