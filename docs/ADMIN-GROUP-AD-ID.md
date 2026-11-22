# Mengubah anggota group AD dari Vikunja

Status: RC6 sudah dibangun, dipindai, dipublikasikan privat dan dipasang pada
instance uji NS8. Panel UI sudah terverifikasi; pemeriksaan izin dan perubahan
AD nyata dengan password pengguna masih menunggu uji interaktif pemilik.
Lihat [hasil validasi dan batas pengujian](VALIDATION.md).
RC5 hanya menyinkronkan AD ke Vikunja.

## Hak yang dipakai

Saat LDAP aktif, teams mengikuti group directory. Hak mengubah anggota diperiksa
oleh directory sebagai **pengguna AD yang sedang login**, bukan berdasarkan label
Admin Team, admin NS8, atau admin seluruh instance Vikunja. Akun harus mempunyai
izin mengubah anggota pada group yang dipilih. Delegasi per group didukung;
keanggotaan pada suatu group saja tidak berarti boleh mengelola anggotanya.

Pada Samba AD, daftar group yang dapat dikelola diperoleh melalui atribut
`allowedAttributesEffective` ketika bind sebagai pengguna tersebut. Directory
memeriksa izin lagi saat perubahan diterapkan. Pada server RFC2307 yang tidak
menyediakan atribut tersebut, keputusan akhir tetap berada pada ACL LDAP saat
operasi Modify; daftar yang terlihat bukan jaminan boleh menulis.

Tidak ada password admin bawaan atau akun bind bersama yang diberi hak tulis.
Gunakan password **akun AD Anda sendiri**, bukan password admin NS8. Password
dipakai dalam permintaan terkait dan tidak disimpan dalam konfigurasi, database,
atau catatan pemulihan modul. Antarmuka mengosongkan field setelah permintaan.

## Langkah menambah anggota

1. Pastikan NS8 Settings sudah memilih domain LDAP, atribut email yang benar,
   dan **Automatically sync LDAP users and groups as teams** aktif.
2. Login ke Vikunja menggunakan akun AD Anda.
3. Buka **Teams**. Panel **Manage directory group members** juga tersedia saat
   membuka sebuah team LDAP.
4. Masukkan password AD pada **Your directory password**, lalu klik
   **Load groups I can manage**. Ini hanya memeriksa dan membaca; belum mengubah AD.
5. Pilih group yang akan dikelola. Dari halaman team tertentu, group tersebut
   dipilih bila Anda memang mempunyai izin mengelolanya.
6. Cari pengguna melalui nama, username, atau email lengkap. Klik hasil
   autocomplete agar pengguna terpilih, lalu klik **Add to Team**.
7. Periksa nama pengguna dan group pada konfirmasi. Masukkan password AD Anda
   dan klik **Confirm directory change**.
8. Setelah directory mengonfirmasi, keanggotaan Vikunja diperbarui langsung.
   Perubahan AD dari alat lain juga diperiksa oleh timer sekitar setiap 15 menit.

Pengguna yang akan dipilih harus sudah memiliki profil LDAP yang valid di
Vikunja. Provisioning otomatis membuatnya sebelum login pertama, asalkan akun
aktif/terlihat dan memiliki email valid. Untuk akun baru, isi Email ketika membuat
akun di NS8; ketersediaan mailbox tidak otomatis mengisi atribut `mail` AD.

## Langkah menghapus anggota

1. Muat group yang dapat Anda kelola seperti langkah di atas.
2. Pilih **Remove** pada pengguna dengan jenis keanggotaan **Direct**.
3. Periksa konfirmasi pengguna/group, masukkan password AD, lalu konfirmasi.

**Primary group** dan **Inherited** bukan nilai anggota langsung yang dapat
dihapus dari group tersebut. Ubah primary group melalui administrasi user AD;
ubah sumber group bertingkat untuk keanggotaan inherited. Jika seseorang punya
jalur keanggotaan lain, menghapus jalur langsung dapat tetap menyisakan akses
melalui group lain. UI melaporkan kondisi itu setelah membaca ulang directory.

Perubahan hanya menambah/menghapus satu nilai `member` atau `memberUid`, sehingga
anggota lain tidak diganti dengan hasil snapshot lama. Group dan user diperiksa
dengan identitas stabil sebelum menulis. Group baru, rename, dan penghapusan
group tetap dilakukan melalui administrasi directory.

## Teams, projects, dan mode lokal

Saat LDAP aktif, jalur perubahan team lokal diblokir agar membership tidak
berbeda dari AD. Tim manual lama dipertahankan sebagai data lama; tidak diambil
alih menjadi group AD. Saat LDAP tidak digunakan, teams kembali dikelola secara
lokal melalui alur Vikunja biasa.

Project tetap dibuat dan dikelola di Vikunja. Menambah anggota project atau
memberikan akses project ke team tidak mengubah group AD. Jika project dibagikan
ke team AD, perubahan anggota group menentukan akses melalui team itu sesuai
mekanisme berbagi Vikunja. Hak admin project tidak diberikan otomatis dari AD.

Kedua opsi Privacy untuk pencarian nama/email lengkap tetap wajib aktif dan
terkunci. Hak admin global Vikunja Pro dan lisensinya tidak diubah oleh fitur ini.

## Jika perubahan belum terlihat

- Pastikan menggunakan akun AD dengan izin mengubah anggota group tersebut.
  Izin Admin Team Vikunja saja tidak cukup.
- Muat ulang group dengan **Load groups I can manage** untuk membaca izin
  terkini, terutama setelah delegasi AD atau password berubah.
- Respons penolakan directory tidak memberi hak tambahan dan tidak menggunakan
  akun bind bersama sebagai pengganti pengguna.
- Bila directory sudah diperbarui tetapi refresh Vikunja tertunda, UI menyatakan
  kondisi tersebut. Muat ulang dan tinjau membership sebelum mengulangi perubahan.
- Permintaan dibatasi 10 pemeriksaan/perubahan per menit untuk tiap akun.
  Token API dan MCP tidak dapat memakai jalur perubahan directory ini.

## Backup dan pemulihan

Buat backup NS8 sebelum update kandidat. Sebelum setiap perubahan AD, modul
menyimpan intent pemulihan yang berisi GUID group/user serta nilai keanggotaan
sebelum/sesudah, tanpa password. Catatan tersimpan dalam tabel privat
`ns8_directory_member_changes` dan ikut dump database pada backup native NS8.

AD dan database aplikasi bukan satu transaksi. Jika koneksi putus setelah Modify,
modul membaca ulang directory untuk mengonfirmasi hasil. Jika hasil belum dapat
dikonfirmasi, UI meminta pemeriksaan ulang dan intent tetap disimpan. Tidak ada
rollback otomatis yang dapat menimpa perubahan admin lain.

Untuk membatalkan perubahan yang telah dikonfirmasi, gunakan operasi kebalikan
setelah memastikan GUID dan membership terkini masih sesuai catatan. Jangan
memulihkan seluruh AD atau mengganti seluruh daftar anggota demi membatalkan
satu perubahan. Backup database Vikunja tidak membatalkan perubahan yang sudah
diterapkan ke AD.

Referensi: [izin efektif atribut AD](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-adts/c16a23d1-6847-4b25-9f6b-f09fc29e8627),
[primary group AD](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-ada3/e12954a4-6865-4432-94e6-00c310ca87c0),
[panduan LDAP dan Privacy modul](LDAP-DAN-AKSES-NS8-ID.md).
