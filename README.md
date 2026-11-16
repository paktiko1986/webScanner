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
