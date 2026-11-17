# 🛡️ WebSCANNER

![Version](https://img.shields.io/badge/version-v2026.11.16-blue)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)

**WebSCANNER** adalah *toolkit recon* dan pemindaian kerentanan web berkecepatan tinggi yang ditulis dalam **Go**. Dirancang untuk membantu *bug bounty hunter*, *pentester*, dan admin sistem dalam memetakan *attack surface* secara otomatis dan akurat.

## ✨ Fitur Utama

- **🔍 Tech Fingerprinting**: Mendeteksi *framework*, CMS (WordPress, Drupal), Web Server (Nginx, Apache), dan CDN (Cloudflare, Akamai).
- **🧩 NVD CVE Checker**: Terintegrasi dengan API NVD (National Vulnerability Database) untuk mengecek kerentanan pada versi PHP, Nginx, atau Apache yang terdeteksi. *(Butuh API Key)*.
- **🔑 Secret Scanner**: Mencari API Key, Token, JWT, dan *password* yang bocor di HTML maupun file JavaScript.
- **📂 Common Exposures**: Memeriksa file sensitif yang sering terekspos seperti `.env`, `.git/config`, `backup.sql`, dll.
- **🌐 CORS & WAF Detection**: Menganalisis salah konfigurasi CORS (termasuk *preflight*) dan mendeteksi keberadaan WAF (Cloudflare, Sucuri, ModSecurity).
- **🚨 Subdomain Takeover**: Mendeteksi potensi *takeover* pada subdomain yang mengarah ke layanan pihak ketiga (AWS S3, GitHub Pages, Heroku, dll).
- **⚙️ Parameter Discovery**: (Jika `-fuzz` aktif) Mengekstrak nama parameter dari form HTML dan mem-*fuzz* parameter tersebut untuk mencari anomali.
- **📧 DNS Security**: Memeriksa record SPF, DMARC, dan DKIM untuk mencegah *email spoofing*.
- **📄 Report Generation**: Menghasilkan laporan detail dalam format **JSON**, **HTML** (dengan UI terminal interaktif), dan ringkasan **Terminal berwarna**.

## 🚀 Instalasi

Pastikan Anda sudah menginstal **Go** (versi 1.21 atau lebih baru).

```bash
# Clone repository ini
git clone https://github.com/paktiko1986/webScanner.git
cd webScanner

# Download dependencies
go mod tidy

# Build binary
go build -o webscanner main.go
```

## 🛠️ Penggunaan Dasar

```bash
# Scan satu target
./webscanner -url https://example.com

# Scan banyak target dari file
./webscanner -list livehosts.txt

# Scan dengan output HTML dan JSON
./webscanner -list livehosts.txt -html report.html -json report.json

# Scan dengan concurrency lebih tinggi (default 3)
./webscanner -list livehosts.txt -concurrency 10

# Hanya tampilkan host yang berpotensi berbahaya (skor < 70)
./webscanner -list livehosts.txt -only-interesting
```

### 🔑 Mengaktifkan NVD CVE Checker

Untuk mengecek CVE berdasarkan versi teknologi yang ditemukan, Anda memerlukan API Key gratis dari [NVD](https://nvd.nist.gov/developers/request-an-api-key).

```bash
# Set API Key di environment variable (JANGAN di-hardcode!)
export NVD_API_KEY="API_KEY_ANDA_DI_SINI"

# Jalankan scanner dengan flag -cve
./webscanner -list livehosts.txt -cve
```

### 📌 Daftar Flag Lengkap

| Flag | Deskripsi | Default |
|------|-----------|---------|
| `-url` | Target URL tunggal | - |
| `-list` | File berisi daftar URL (satu per baris) | - |
| `-html` | Simpan laporan ke file HTML | - |
| `-json` | Simpan hasil ke file JSON | - |
| `-cve` | Aktifkan NVD CVE check (butuh `NVD_API_KEY`) | `false` |
| `-fuzz` | Aktifkan parameter discovery | `false` |
| `-no-js` | Lewati ekstraksi file JS (lebih cepat) | `false` |
| `-quiet` | Hanya tampilkan progress bar & ringkasan | `false` |
| `-concurrency` | Jumlah worker paralel | `3` |
| `-timeout` | Timeout request (detik) | `15` |
| `-only-interesting` | Hanya tampilkan host dengan skor < 70 | `false` |
| `-resume` | Lanjutkan scan dari `progress-file` | `false` |
| `-no-color` | Matikan output berwarna | `false` |

## 📸 Contoh Output

### 1. Terminal UI
```text
┌── 🔒 SSL DIAGNOSIS ─────────────────────────────────────────
│ Host                      : example.com
│ Status                    : OK (chain valid)
└─────────────────────────────────────────────────────────────

┌── ⚙️  TECH FINGERPRINT ─────────────────────────────────────
│ language                  : PHP 8.2.30
│ server                    : Nginx 1.18.0
└─────────────────────────────────────────────────────────────

┌── 🧩 NVD CVE CHECKER (PHP 8.2.30) ─────────────────────────
│ CVE-2026-109618           : [CRITICAL 9.8] PHP2Shell RCE ...
└─────────────────────────────────────────────────────────────
```

### 2. HTML Report
Laporan HTML menyediakan antarmuka visual yang rapi dengan kotak-kotak terminal yang menampilkan detail setiap temuan.

## ⚠️ Disclaimer

*Tool* ini dibuat untuk tujuan **edukasi dan pengujian keamanan yang sah**.
- **JANGAN** menggunakan *tool* ini pada sistem yang tidak Anda miliki atau tanpa izin tertulis.
- Penggunaan *tool* ini untuk aktivitas ilegal adalah pelanggaran hukum dan bukan tanggung jawab pengembang.

## 👨‍💻 Author

**Santiko Kusnul Hakim**
- GitHub: [@paktiko1986](https://github.com/paktiko1986)

## 📝 Lisensi
Proyek ini dilisensikan di bawah [MIT License](https://mit-license.org/).
