package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/schollz/progressbar/v3"
)

// ============================================================
// WARNA ANSI & UI HELPER
// ============================================================
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[1;91m"
	colorGreen  = "\033[1;32m"
	colorYellow = "\033[1;93m"
	colorCyan   = "\033[1;36m"
	colorGray   = "\033[90m"
	colorWhite  = "\033[97m"
	colorBold   = "\033[1m"
)

func colorize(color, s string) string {
	if os.Getenv("NO_COLOR") != "" { return s }
	return color + s + colorReset
}

func printSection(title string) {
	if os.Getenv("NO_COLOR") != "" { fmt.Printf("\n--- %s ---\n", title); return }
	line := strings.Repeat("─", 68-len(title))
	fmt.Printf("\n%s %s %s\n", colorize(colorGray, "┌──"), colorize(colorCyan+colorBold, title), colorize(colorGray, line))
}

func printField(label, value string, statusColor string) {
	if os.Getenv("NO_COLOR") != "" { fmt.Printf("  %-25s : %s\n", label, value); return }
	fmt.Printf("%s %-25s : %s\n", colorize(colorGray, "│"), colorize(colorGray, label), colorize(statusColor, value))
}

func printStatusBar(isOK bool, label, note string) {
	if os.Getenv("NO_COLOR") != "" { status := "[ OK ]"; if !isOK { status = "[FAIL]" }; fmt.Printf("  %s %s %s\n", status, label, note); return }
	if isOK {
		fmt.Printf("%s %s %s %s\n", colorize(colorGray, "│"), colorize(colorGreen, "[ OK ]"), label, colorize(colorGray, note))
	} else {
		fmt.Printf("%s %s %s %s\n", colorize(colorGray, "│"), colorize(colorRed, "[FAIL]"), colorize(colorWhite+colorBold, label), colorize(colorYellow, note))
	}
}

func printFooter() {
	if os.Getenv("NO_COLOR") == "" { fmt.Printf("%s\n", colorize(colorGray, "└"+strings.Repeat("─", 68))) }
}

func printBanner() {
	if os.Getenv("NO_COLOR") != "" {
		fmt.Println("========================================")
		fmt.Println(" app     : WebSCANNER")
		fmt.Println(" version : v2026.11.16")
		fmt.Println(" author  : Santiko Kusnul Hakim")
		fmt.Println("========================================")
		return
	}
	banner := `
 ██╗    ██╗███████╗██████╗ ███████╗ ██████╗ █████╗ ███╗   ██╗███╗   ██╗███████╗██████╗ 
 ██║    ██║██╔════╝██╔══██╗██╔════╝██╔════╝██╔══██╗████╗  ██║████╗  ██║██╔════╝██╔══██╗
 ██║ █╗ ██║█████╗  ██████╔╝███████╗██║     ███████║██╔██╗ ██║██╔██╗ ██║█████╗  ██████╔╝
 ██║███╗██║██╔══╝  ██╔══██╗╚════██║██║     ██╔══██║██║╚██╗██║██║╚██╗██║██╔══╝  ██╔══██╗
 ╚███╔███╔╝███████╗██████╔╝███████║╚██████╗██║  ██║██║ ╚████║██║ ╚████║███████╗██║  ██║
  ╚══╝╚══╝ ╚══════╝╚═════╝ ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═══╝╚══════╝╚═╝  ╚═╝
`
	fmt.Println(colorize(colorCyan+colorBold, banner))
	fmt.Printf("  %s : %s\n", colorize(colorGray, "app    "), colorize(colorWhite, "WebSCANNER"))
	fmt.Printf("  %s : %s\n", colorize(colorGray, "version"), colorize(colorGreen, "v2026.11.16"))
	fmt.Printf("  %s : %s\n", colorize(colorGray, "author "), colorize(colorYellow, "Santiko Kusnul Hakim"))
	fmt.Println(colorize(colorGray, "  ─────────────────────────────────────────────────────────"))
}

// ============================================================
// KONFIGURASI GLOBAL
// ============================================================
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"

var exposurePaths = []struct {
	Path     string
	Category string
	Validate func(body string, headers http.Header) bool
}{
	{"/.env", "ENV", func(b string, _ http.Header) bool { return regexp.MustCompile(`(?m)^[A-Z_]+=`).MatchString(b) && !strings.Contains(strings.ToLower(b), "<html") }},
	{"/.env.local", "ENV", func(b string, _ http.Header) bool { return regexp.MustCompile(`(?m)^[A-Z_]+=`).MatchString(b) }},
	{"/.env.production", "ENV", func(b string, _ http.Header) bool { return regexp.MustCompile(`(?m)^[A-Z_]+=`).MatchString(b) }},
	{"/.env.backup", "ENV", func(b string, _ http.Header) bool { return regexp.MustCompile(`(?m)^[A-Z_]+=`).MatchString(b) }},
	{"/.git/config", "GIT", func(b string, _ http.Header) bool { return strings.Contains(b, "[core]") && strings.Contains(b, "repositoryformatversion") }},
	{"/.git/HEAD", "GIT", func(b string, _ http.Header) bool { return strings.HasPrefix(b, "ref: refs/") }},
	{"/.git/index", "GIT", func(b string, _ http.Header) bool { return strings.HasPrefix(b, "DIRC") }},
	{"/.git/logs/HEAD", "GIT", func(b string, _ http.Header) bool { return strings.Contains(b, "commit") }},
	{"/.svn/entries", "SVN", func(b string, _ http.Header) bool { return strings.Contains(b, "dir") }},
	{"/.svn/wc.db", "SVN", func(b string, _ http.Header) bool { return strings.HasPrefix(b, "SQLite format 3") }},
	{"/.hg/requires", "HG", func(b string, _ http.Header) bool { return strings.Contains(b, "revlogv1") }},
	{"/index.php.bak", "BACKUP", func(b string, _ http.Header) bool { return strings.Contains(b, "<?php") }},
	{"/index.php~", "BACKUP", func(b string, _ http.Header) bool { return strings.Contains(b, "<?php") }},
	{"/config.php.bak", "BACKUP", func(b string, _ http.Header) bool { return strings.Contains(b, "<?php") }},
	{"/wp-config.php.bak", "BACKUP", func(b string, _ http.Header) bool { return strings.Contains(b, "DB_NAME") }},
	{"/db.sql", "BACKUP", func(b string, _ http.Header) bool { return strings.Contains(b, "CREATE TABLE") }},
	{"/db.sqlite", "BACKUP", func(b string, _ http.Header) bool { return strings.HasPrefix(b, "SQLite format 3") }},
	{"/database.sql", "BACKUP", func(b string, _ http.Header) bool { return strings.Contains(b, "CREATE TABLE") }},
	{"/backup.sql", "BACKUP", func(b string, _ http.Header) bool { return strings.Contains(b, "CREATE TABLE") }},
	{"/backup.zip", "BACKUP", func(b string, h http.Header) bool { return strings.Contains(h.Get("Content-Type"), "zip") }},
	{"/backup.tar.gz", "BACKUP", func(b string, h http.Header) bool { return strings.Contains(h.Get("Content-Type"), "gzip") }},
	{"/.DS_Store", "MISC", func(b string, _ http.Header) bool { return strings.HasPrefix(b, "\x00\x00\x00\x01Bud1") }},
	{"/.aws/credentials", "CLOUD", func(b string, _ http.Header) bool { return strings.Contains(b, "aws_access_key_id") }},
	{"/phpinfo.php", "MISC", func(b string, _ http.Header) bool { return strings.Contains(b, "PHP Version") }},
	{"/.htaccess", "MISC", func(b string, _ http.Header) bool { return strings.Contains(b, "RewriteEngine") }},
}

var takeoverSignatures = []struct {
	Service    string
	Regex      *regexp.Regexp
	CNAMES     []string
	Confidence string
}{
	{"AWS S3", regexp.MustCompile(`(?i)(NoSuchBucket|The specified bucket does not exist)`), []string{"s3.amazonaws.com", "s3-website"}, "HIGH"},
	{"GitHub Pages", regexp.MustCompile(`(?i)There isn't a GitHub Pages site here`), []string{"github.io"}, "HIGH"},
	{"Heroku", regexp.MustCompile(`(?i)(No such app|herokuapp.com)`), []string{"herokuapp.com", "herokussl.com"}, "HIGH"},
	{"Bitbucket", regexp.MustCompile(`(?i)Repository not found`), []string{"bitbucket.io"}, "HIGH"},
	{"Shopify", regexp.MustCompile(`(?i)Sorry, this shop is currently unavailable`), []string{"myshopify.com"}, "HIGH"},
	{"Fastly", regexp.MustCompile(`(?i)Fastly error: unknown domain`), []string{"fastly.net"}, "HIGH"},
	{"CloudFront", regexp.MustCompile(`(?i)(Bad request|ERROR: The request could not be satisfied).*cloudfront`), []string{"cloudfront.net"}, "MEDIUM"},
	{"Azure", regexp.MustCompile(`(?i)(404 Web Site not found|This web app is stopped)`), []string{"azurewebsites.net", "cloudapp.azure.com"}, "HIGH"},
	{"Cargo", regexp.MustCompile(`(?i)If you're moving your domain away from Cargo`), []string{"cargocollective.com"}, "HIGH"},
	{"Tumblr", regexp.MustCompile(`(?i)Whatever you were looking for doesn't currently exist at this address`), []string{"tumblr.com"}, "HIGH"},
	{"WordPress.com", regexp.MustCompile(`(?i)Do you want to register .* wordpress\.com`), []string{"wordpress.com"}, "HIGH"},
	{"Netlify", regexp.MustCompile(`(?i)Not found - Request ID`), []string{"netlify.app", "netlify.com"}, "MEDIUM"},
	{"Surge", regexp.MustCompile(`(?i)project not found`), []string{"surge.sh"}, "HIGH"},
	{"UserVoice", regexp.MustCompile(`(?i)This UserVoice subdomain is currently available!`), []string{"uservoice.com"}, "HIGH"},
	{"Pantheon", regexp.MustCompile(`(?i)The gods are wise, but do not know of the site which you seek`), []string{"pantheonsite.io"}, "HIGH"},
	{"Zendesk", regexp.MustCompile(`(?i)Help Center Closed`), []string{"zendesk.com"}, "HIGH"},
	{"Unbounce", regexp.MustCompile(`(?i)The requested URL was not found on this server`), []string{"unbouncepages.com"}, "MEDIUM"},
	{"ReadTheDocs", regexp.MustCompile(`(?i)Unknown domain`), []string{"readthedocs.io"}, "MEDIUM"},
	{"Statuspage", regexp.MustCompile(`(?i)This page is either not available`), []string{"statuspage.io"}, "MEDIUM"},
	{"Ghost", regexp.MustCompile(`(?i)The thing you were looking for is no longer here`), []string{"ghost.io"}, "HIGH"},
}

var secretNamePattern = regexp.MustCompile(`(?i)\b(api[_-]?key|apikey|secret[_-]?key|secretkey|client[_-]?secret|access[_-]?token|refresh[_-]?token|auth[_-]?token|bearer|private[_-]?key|password|passwd|aws[_-]?access|google[_-]?api|stripe[_-]?|sendgrid|mailgun|twilio|database[_-]?url|db[_-]?url|jwt[_-]?secret)\b`)

var secretValuePatterns = []struct {
	re   *regexp.Regexp
	name string
}{
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "AWS Access Key ID"},
	{regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`), "Google API Key"},
	{regexp.MustCompile(`\bsk_live_[0-9a-zA-Z]{24,}\b`), "Stripe Secret Key"},
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b`), "JWT Token"},
}

var falsePositiveValues = map[string]bool{"": true, "null": true, "undefined": true, "none": true, "false": true, "true": true, "0": true, "1": true, "your_api_key": true, "test": true}

// ============================================================
// STRUKTUR DATA
// ============================================================
type SSLDiag struct { OK bool `json:"ok"`; Host string `json:"host"`; Reason string `json:"reason,omitempty"` }
type SecretFinding struct { Type string `json:"type"`; Name string `json:"name"`; ValueMasked string `json:"value_masked"`; RawValue string `json:"raw_value"`; Source string `json:"source"` }
type WPUser struct { ID interface{} `json:"id"`; Name string `json:"name"`; Slug string `json:"slug"` }
type StaticFindings struct {
	HiddenInputs   []string            `json:"hidden_inputs"`
	Forms          []string            `json:"forms"`
	JSVars         []string            `json:"js_vars"`
	PotentialSinks []string            `json:"potential_sinks"`
	WPSignals      []string            `json:"wp_signals"`
	PluginVersions map[string][]string `json:"plugin_versions"`
	ThemeVersions  map[string][]string `json:"theme_versions"`
	WPVersion      string              `json:"wp_version,omitempty"`
}
type JSEndpoints struct { Endpoints []string `json:"endpoints"`; Actions []string `json:"actions"` }
type HeaderFinding struct { Name string `json:"name"`; Status string `json:"status"`; Value string `json:"value,omitempty"`; Note string `json:"note,omitempty"` }
type CORSFinding struct { OriginHeader string `json:"origin_header,omitempty"`; AllowOrigin string `json:"allow_origin,omitempty"`; AllowCreds string `json:"allow_credentials,omitempty"`; Vulnerable bool `json:"vulnerable"`; Note string `json:"note,omitempty"` }
type ExposureFinding struct { Path string `json:"path"`; Category string `json:"category"`; Status int `json:"status"`; Size int `json:"size"`; Note string `json:"note,omitempty"` }
type TakeoverFinding struct { Service string `json:"service"`; CNAME string `json:"cname,omitempty"`; Evidence string `json:"evidence,omitempty"`; Vulnerable bool `json:"vulnerable"`; Confidence string `json:"confidence,omitempty"` }
type TechFingerprint struct { Name string `json:"name"`; Version string `json:"version,omitempty"`; Category string `json:"category"` }
type DNSSecurity struct { SPF string `json:"spf,omitempty"`; DMARC string `json:"dmarc,omitempty"`; DKIM string `json:"dkim,omitempty"`; Issues []string `json:"issues,omitempty"` }
type Vulnerability struct { ID string `json:"id"`; Title string `json:"title"`; Severity string `json:"severity"` }
type WAFFinding struct { Detected bool `json:"detected"`; Name string `json:"name,omitempty"`; Evidence string `json:"evidence,omitempty"`; RateLimited bool `json:"rate_limited"`; RateLimitNote string `json:"rate_limit_note,omitempty"`; StatusOnXSS int `json:"status_on_xss,omitempty"` }
type CORSPreflight struct { AllowMethods string `json:"allow_methods,omitempty"`; AllowHeaders string `json:"allow_headers,omitempty"`; MaxAge string `json:"max_age,omitempty"`; AllowOrigin string `json:"allow_origin,omitempty"`; Vulnerable bool `json:"vulnerable"`; Note string `json:"note,omitempty"` }
type JWTInfo struct { Raw string `json:"raw"`; Alg string `json:"alg"`; Type string `json:"type"`; Claims []string `json:"claims"`; IsNone bool `json:"is_none"`; IsExpired bool `json:"is_expired"`; Warnings []string `json:"warnings"` }

// NVD Structs
type NVDResponse struct {
	Vulnerabilities []struct {
		CVE struct {
			ID           string `json:"id"`
			Descriptions []struct { Lang string `json:"lang"`; Value string `json:"value"` } `json:"descriptions"`
			Metrics struct {
				CvssMetricV31 []struct { CvssData struct { BaseScore float64 `json:"baseScore"`; BaseSeverity string `json:"baseSeverity"` } `json:"cvssData"` } `json:"cvssMetricV31"`
				CvssMetricV2  []struct { CvssData struct { BaseScore float64 `json:"baseScore"` } `json:"cvssData"`; BaseSeverity string `json:"baseSeverity"` } `json:"cvssMetricV2"`
			} `json:"metrics"`
			Configurations []struct { Nodes []struct { CPEMatch []struct { Criteria string `json:"criteria"` } `json:"cpeMatch"` } `json:"nodes"` } `json:"configurations"`
		} `json:"cve"`
	} `json:"vulnerabilities"`
}

type CVEInfo struct { ID string; Description string; Severity string; Score float64; Versions []string }

type ScanResult struct {
	Target       string            `json:"target"`
	SSL          *SSLDiag          `json:"ssl"`
	HTTPStatus   int               `json:"http_status,omitempty"`
	FinalURL     string            `json:"final_url,omitempty"`
	Size         int               `json:"size,omitempty"`
	Error        string            `json:"error,omitempty"`
	Static       *StaticFindings   `json:"static,omitempty"`
	JS           *JSEndpoints      `json:"js,omitempty"`
	Secrets      []SecretFinding   `json:"secrets"`
	WPUsers      []WPUser          `json:"wp_users"`
	Fuzz         []string          `json:"fuzz"`
	Headers      []HeaderFinding   `json:"headers"`
	CORS         *CORSFinding      `json:"cors,omitempty"`
	Preflight    *CORSPreflight    `json:"cors_preflight,omitempty"`
	Exposures    []ExposureFinding `json:"exposures"`
	Takeover     *TakeoverFinding  `json:"takeover,omitempty"`
	TechStack    []TechFingerprint `json:"tech_stack,omitempty"`
	DNS          *DNSSecurity      `json:"dns,omitempty"`
	WAF          *WAFFinding       `json:"waf,omitempty"`
	JWTs         []*JWTInfo        `json:"jwts,omitempty"`
	WPVulns      []string          `json:"wp_vulns"`
	TechVulns    []CVEInfo         `json:"tech_vulns,omitempty"`
	Score        int               `json:"score"`
	ScoreReasons []string          `json:"score_reasons"`
	DurationMS   int64             `json:"duration_ms"`
}

// ============================================================
// HTTP CLIENT
// ============================================================
func newHTTPClient(timeout int, insecure bool) *http.Client {
	return &http.Client{
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure}, MaxIdleConns: 100, IdleConnTimeout: 30 * time.Second},
		CheckRedirect: func(req *http.Request, via []*http.Request) error { if len(via) >= 10 { return fmt.Errorf("too many redirects") }; return nil },
	}
}

func fetchSource(client *http.Client, target string) (string, bool, int, string, http.Header) {
	req, _ := http.NewRequest("GET", target, nil)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		insecureClient := newHTTPClient(15, true)
		resp2, err2 := insecureClient.Do(req)
		if err2 != nil { return "", false, 0, "", nil }
		defer resp2.Body.Close()
		body, _ := io.ReadAll(resp2.Body)
		return string(body), false, resp2.StatusCode, resp2.Request.URL.String(), resp2.Header
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), true, resp.StatusCode, resp.Request.URL.String(), resp.Header
}

// ============================================================
// NVD API CVE CHECKER
// ============================================================
func queryNVD(product, version string) []CVEInfo {
	apiKey := os.Getenv("NVD_API_KEY")
	if apiKey == "" {
		fmt.Println("[!] NVD_API_KEY tidak diset. Melewati CVE check.")
		return nil
	}
	cpeKeyword := fmt.Sprintf("cpe:2.3:a:%s:%s", strings.ToLower(product), strings.ToLower(product))
	encodedKeyword := url.QueryEscape(cpeKeyword)
	apiURL := fmt.Sprintf("https://services.nvd.nist.gov/rest/json/cves/2.0?cpeName=%s", encodedKeyword)
	
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("apiKey", apiKey)
	
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 { return nil }
	defer resp.Body.Close()

	var nvdResp NVDResponse
	if err := json.NewDecoder(resp.Body).Decode(&nvdResp); err != nil { return nil }

	var results []CVEInfo
	for _, v := range nvdResp.Vulnerabilities {
		desc := ""
		for _, d := range v.CVE.Descriptions { if d.Lang == "en" { desc = d.Value; break } }
		severity := "UNKNOWN"
		score := 0.0
		if len(v.CVE.Metrics.CvssMetricV31) > 0 { severity = v.CVE.Metrics.CvssMetricV31[0].CvssData.BaseSeverity; score = v.CVE.Metrics.CvssMetricV31[0].CvssData.BaseScore } else if len(v.CVE.Metrics.CvssMetricV2) > 0 { severity = v.CVE.Metrics.CvssMetricV2[0].BaseSeverity; score = v.CVE.Metrics.CvssMetricV2[0].CvssData.BaseScore }

		var versions []string
		for _, conf := range v.CVE.Configurations {
			for _, node := range conf.Nodes {
				for _, match := range node.CPEMatch {
					parts := strings.Split(match.Criteria, ":")
					if len(parts) >= 6 && parts[5] != "*" && parts[5] != "-" { versions = append(versions, parts[5]) }
				}
			}
		}
		
		isVulnerable := false
		for _, v := range versions { if v == version { isVulnerable = true; break } }
		if isVulnerable {
			results = append(results, CVEInfo{ID: v.CVE.ID, Description: desc, Severity: severity, Score: score, Versions: versions})
		}
	}
	return results
}

func printNVDResults(product, version string, cves []CVEInfo) {
	if len(cves) == 0 { return }
	printSection(fmt.Sprintf("🧩 NVD CVE CHECKER (%s %s)", product, version))
	for _, c := range cves {
		color := colorRed
		if c.Severity == "MEDIUM" { color = colorYellow } else if c.Severity == "LOW" { color = colorGray }
		printField(c.ID, fmt.Sprintf("[%s %.1f] %s", c.Severity, c.Score, c.Description), color)
	}
	printFooter()
}

// ============================================================
// SSL, HEADERS, CORS, WAF, EXPOSURES, TAKEOVER, DNS, SECRETS
// ============================================================
// (Fungsi-fungsi ini sama persis dengan versi sebelumnya, tidak diubah)
func diagnoseSSL(target string, timeout int) *SSLDiag { u, _ := url.Parse(target); host := u.Hostname(); dialer := &net.Dialer{Timeout: time.Duration(timeout) * time.Second}; conn, err := tls.DialWithDialer(dialer, "tcp", host+":443", &tls.Config{ServerName: host}); if err != nil { reason := err.Error(); if strings.Contains(reason, "unknown authority") || strings.Contains(reason, "local issuer") || strings.Contains(reason, "certificate signed") { return &SSLDiag{OK: false, Host: host, Reason: "Verifikasi GAGAL: " + reason} }; if strings.Contains(strings.ToLower(reason), "expired") { return &SSLDiag{OK: false, Host: host, Reason: "Verifikasi GAGAL: sertifikat kadaluarsa"} }; return &SSLDiag{OK: false, Host: host, Reason: reason} }; defer conn.Close(); return &SSLDiag{OK: true, Host: host} }
func printSSLDiagnosis(diag *SSLDiag) { printSection("🔒 SSL DIAGNOSIS"); printField("Host", diag.Host, colorWhite); if diag.OK { printField("Status", "OK (chain valid)", colorGreen) } else { printField("Status", "GAGAL", colorRed); printField("Reason", diag.Reason, colorYellow) }; printFooter() }
func auditHeaders(h http.Header) []HeaderFinding { out := []HeaderFinding{}; check := func(name string, validator func(v string) (string, string)) { v := h.Get(name); if v == "" { out = append(out, HeaderFinding{Name: name, Status: "missing", Note: "header tidak ada"}); return }; status, note := validator(v); out = append(out, HeaderFinding{Name: name, Status: status, Value: v, Note: note}) }; check("Strict-Transport-Security", func(v string) (string, string) { if strings.Contains(v, "max-age=") { return "ok", "" }; return "weak", "max-age tidak diset" }); check("Content-Security-Policy", func(v string) (string, string) { if strings.Contains(v, "unsafe-inline") || strings.Contains(v, "unsafe-eval") { return "weak", "unsafe" }; return "ok", "" }); check("X-Frame-Options", func(v string) (string, string) { lv := strings.ToUpper(v); if lv == "DENY" || lv == "SAMEORIGIN" { return "ok", "" }; return "weak", "nilai tidak standar" }); check("X-Content-Type-Options", func(v string) (string, string) { if strings.ToLower(v) == "nosniff" { return "ok", "" }; return "weak", "harus nosniff" }); check("Referrer-Policy", func(v string) (string, string) { lv := strings.ToLower(v); if lv == "no-referrer" || lv == "strict-origin-when-cross-origin" || lv == "same-origin" { return "ok", "" }; return "weak", "nilai lemah" }); check("Permissions-Policy", func(v string) (string, string) { return "ok", "" }); if s := h.Get("Server"); s != "" { note := "info disclosure"; if regexp.MustCompile(`[0-9]+\.[0-9]+`).MatchString(s) { note = "versi terekspos" }; out = append(out, HeaderFinding{Name: "Server", Status: "info", Value: s, Note: note}) }; if s := h.Get("X-Powered-By"); s != "" { out = append(out, HeaderFinding{Name: "X-Powered-By", Status: "info", Value: s, Note: "info disclosure"}) }; return out }
func printHeaders(findings []HeaderFinding) { printSection("🛡️  SECURITY HEADERS"); for _, f := range findings { switch f.Status { case "missing": printStatusBar(false, f.Name, "MISSING"); case "weak": printStatusBar(false, f.Name, "WEAK: "+f.Value); case "info": printStatusBar(true, f.Name, "INFO: "+f.Value); default: printStatusBar(true, f.Name, "") } }; printFooter() }
func checkCORS(client *http.Client, target string) *CORSFinding { req, _ := http.NewRequest("GET", target, nil); req.Header.Set("User-Agent", userAgent); req.Header.Set("Origin", "https://evil.example.com"); resp, err := client.Do(req); if err != nil { return &CORSFinding{} }; defer resp.Body.Close(); f := &CORSFinding{OriginHeader: "https://evil.example.com", AllowOrigin: resp.Header.Get("Access-Control-Allow-Origin"), AllowCreds: resp.Header.Get("Access-Control-Allow-Credentials")}; if f.AllowOrigin == "" { f.Note = "tidak ada CORS header"; return f }; if f.AllowOrigin == "https://evil.example.com" { f.Vulnerable = true; f.Note = "reflected origin" } else if f.AllowOrigin == "*" && strings.EqualFold(f.AllowCreds, "true") { f.Vulnerable = true; f.Note = "wildcard + credentials" } else if f.AllowOrigin == "null" { f.Vulnerable = true; f.Note = "origin: null" } else { f.Note = "origin: " + f.AllowOrigin }; return f }
func printCORS(c *CORSFinding) { printSection("🌐 CORS CHECK (GET)"); if c == nil || c.AllowOrigin == "" { printField("Status", "Tidak ada CORS header — aman", colorGreen) } else if c.Vulnerable { printField("Status", "VULNERABLE", colorRed); printField("Allow-Origin", c.AllowOrigin, colorRed); printField("Allow-Creds", c.AllowCreds, colorRed) } else { printField("Allow-Origin", c.AllowOrigin, colorGreen) }; printFooter() }
func checkCORSPreflight(client *http.Client, target string) *CORSPreflight { p := &CORSPreflight{}; req, _ := http.NewRequest("OPTIONS", target, nil); req.Header.Set("User-Agent", userAgent); req.Header.Set("Origin", "https://evil.example.com"); req.Header.Set("Access-Control-Request-Method", "POST"); req.Header.Set("Access-Control-Request-Headers", "content-type,x-custom-header"); resp, err := client.Do(req); if err != nil { return p }; defer resp.Body.Close(); p.AllowMethods = resp.Header.Get("Access-Control-Allow-Methods"); p.AllowHeaders = resp.Header.Get("Access-Control-Allow-Headers"); p.MaxAge = resp.Header.Get("Access-Control-Max-Age"); p.AllowOrigin = resp.Header.Get("Access-Control-Allow-Origin"); if p.AllowOrigin == "https://evil.example.com" || p.AllowOrigin == "*" { if strings.Contains(p.AllowMethods, "POST") || strings.Contains(p.AllowMethods, "PUT") || strings.Contains(p.AllowMethods, "DELETE") { p.Vulnerable = true; p.Note = "Origin reflected + method berbahaya diizinkan" } }; if p.MaxAge != "" { if sec, _ := strconv.Atoi(p.MaxAge); sec > 86400 { p.Note += " | Max-Age > 24h" } }; return p }
func printPreflight(p *CORSPreflight) { printSection("🌐 CORS PREFLIGHT (OPTIONS)"); if p == nil || (p.AllowOrigin == "" && p.AllowMethods == "") { printField("Status", "Tidak ada response preflight", colorGray); printFooter(); return }; c := colorGreen; if p.Vulnerable { c = colorRed }; printField("Allow-Origin", p.AllowOrigin, c); printField("Allow-Methods", p.AllowMethods, colorWhite); printField("Allow-Headers", p.AllowHeaders, colorGray); printField("Max-Age", p.MaxAge, colorGray); if p.Vulnerable { printField("Status", "VULNERABLE — "+p.Note, colorRed) } else { printField("Status", "Aman", colorGreen) }; printFooter() }
func detectWAF(client *http.Client, target string) *WAFFinding { w := &WAFFinding{}; xssPayload := "<script>alert('waf-probe')</script>"; testURL := target + "?q=" + url.QueryEscape(xssPayload); req, _ := http.NewRequest("GET", testURL, nil); req.Header.Set("User-Agent", userAgent); resp, err := client.Do(req); if err == nil { defer resp.Body.Close(); w.StatusOnXSS = resp.StatusCode; body, _ := io.ReadAll(resp.Body); bodyStr := string(body); if resp.StatusCode == 403 || resp.StatusCode == 406 || resp.StatusCode == 429 { w.Detected = true; w.Evidence = fmt.Sprintf("HTTP %d saat XSS payload", resp.StatusCode); w.Name = detectWAFName(resp.Header, bodyStr) } else if regexp.MustCompile(`(?i)(cloudflare|modsecurity|akamai|incapsula|sucuri|barracuda|f5|imperva)`).MatchString(bodyStr) { w.Detected = true; w.Name = detectWAFName(resp.Header, bodyStr); w.Evidence = "Signature WAF di body" } }; client2 := newHTTPClient(5, true); start := time.Now(); statuses := []int{}; for i := 0; i < 10; i++ { req, _ := http.NewRequest("GET", target, nil); req.Header.Set("User-Agent", userAgent); if r, err := client2.Do(req); err == nil { statuses = append(statuses, r.StatusCode); r.Body.Close() } }; elapsed := time.Since(start); rateLimitCount := 0; for _, s := range statuses { if s == 429 || s == 503 { rateLimitCount++ } }; if rateLimitCount > 0 { w.RateLimited = true; w.RateLimitNote = fmt.Sprintf("%d/%d request di-rate-limit (%.1fs)", rateLimitCount, len(statuses), elapsed.Seconds()) } else if elapsed > 5*time.Second { w.RateLimited = true; w.RateLimitNote = fmt.Sprintf("Response melambat: 10 req dalam %.1fs", elapsed.Seconds()) }; return w }
func detectWAFName(h http.Header, body string) string { if h.Get("cf-ray") != "" || strings.Contains(body, "cloudflare") { return "Cloudflare" }; if h.Get("x-akamai-request-id") != "" { return "Akamai" }; if h.Get("x-sucuri-id") != "" || strings.Contains(body, "sucuri") { return "Sucuri" }; if h.Get("x-fastly-request-id") != "" { return "Fastly" }; if strings.Contains(body, "mod_security") || strings.Contains(body, "ModSecurity") { return "ModSecurity" }; if strings.Contains(body, "Incapsula") { return "Incapsula" }; if strings.Contains(body, "Imperva") { return "Imperva" }; if strings.Contains(body, "Barracuda") { return "Barracuda" }; if strings.Contains(body, "F5 Networks") { return "F5 BIG-IP" }; if h.Get("x-amz-cf-id") != "" { return "CloudFront" }; return "Unknown WAF" }
func printWAF(w *WAFFinding) { printSection("🛡️  WAF & RATE LIMIT"); if w == nil { printField("Status", "Tidak dapat dideteksi", colorGray) } else if w.Detected { printField("WAF Terdeteksi", w.Name, colorYellow); printField("Evidence", w.Evidence, colorGray); printField("HTTP on XSS", fmt.Sprintf("%d", w.StatusOnXSS), colorYellow) } else { printField("WAF", "Tidak terdeteksi", colorGreen) }; if w != nil && w.RateLimited { printField("Rate Limit", "YA", colorYellow); printField("Note", w.RateLimitNote, colorGray) } else { printField("Rate Limit", "Tidak signifikan", colorGreen) }; printFooter() }
func checkExposures(client *http.Client, baseURL string) []ExposureFinding { printSection("📂 COMMON EXPOSURE FILES"); out := []ExposureFinding{}; u, _ := url.Parse(baseURL); root := u.Scheme + "://" + u.Host; found := false; for _, ep := range exposurePaths { full := root + ep.Path; req, _ := http.NewRequest("GET", full, nil); req.Header.Set("User-Agent", userAgent); resp, err := client.Do(req); if err != nil { continue }; body, _ := io.ReadAll(resp.Body); resp.Body.Close(); if resp.StatusCode != 200 { continue }; if ep.Validate(string(body), resp.Header) { found = true; note := fmt.Sprintf("terekspos (HTTP 200, %d bytes)", len(body)); if len(body) > 500_000 { note = "terekspos (Dump besar)" }; out = append(out, ExposureFinding{Path: ep.Path, Category: ep.Category, Status: resp.StatusCode, Size: len(body), Note: note}); printField(ep.Category+": "+ep.Path, "TEREKSPOS", colorRed) } }; if !found { printField("Status", "Tidak ada file ekspos terdeteksi", colorGreen) }; printFooter(); return out }
func checkTakeover(target string, source string) *TakeoverFinding { u, _ := url.Parse(target); host := u.Hostname(); cname, _ := net.LookupCNAME(host); for _, sig := range takeoverSignatures { if sig.Regex.MatchString(source) { evidence := sig.Regex.FindString(source); confidence := sig.Confidence; for _, cn := range sig.CNAMES { if strings.Contains(cname, cn) { confidence = "HIGH"; break } }; return &TakeoverFinding{Service: sig.Service, CNAME: cname, Evidence: evidence, Vulnerable: true, Confidence: confidence} } }; if cname != "" && cname != host && !strings.HasSuffix(cname, host) { for _, sig := range takeoverSignatures { for _, cn := range sig.CNAMES { if strings.Contains(cname, cn) { return &TakeoverFinding{Service: sig.Service, CNAME: cname, Vulnerable: false, Confidence: "LOW", Evidence: "CNAME ke " + sig.Service} } } } }; return nil }
func printTakeover(t *TakeoverFinding) { printSection("🚨 SUBDOMAIN TAKEOVER"); if t == nil { printField("Status", "Tidak ada indikasi takeover", colorGreen) } else if t.Vulnerable { c := colorRed; if t.Confidence == "MEDIUM" { c = colorYellow } else if t.Confidence == "LOW" { c = colorGray }; printField("Status", "POTENSIAL TAKEOVER ("+t.Confidence+")", c); printField("Service", t.Service, colorYellow); printField("CNAME", t.CNAME, colorWhite); printField("Evidence", t.Evidence, colorGray) } else { printField("Status", "Potensial (CNAME ke "+t.Service+", confidence LOW)", colorYellow) }; printFooter() }
func fingerprintTech(source string, headers http.Header) []TechFingerprint { var techs []TechFingerprint; seen := map[string]bool{}; add := func(name, version, category string) { key := name + "|" + version; if seen[key] { return }; seen[key] = true; techs = append(techs, TechFingerprint{Name: name, Version: version, Category: category}) }; server := strings.ToLower(headers.Get("Server")); if strings.Contains(server, "nginx") { add("Nginx", extractVersion(headers.Get("Server")), "server") }; if strings.Contains(server, "apache") { add("Apache", extractVersion(headers.Get("Server")), "server") }; if strings.Contains(server, "iis") || strings.Contains(server, "microsoft-iis") { add("IIS", extractVersion(headers.Get("Server")), "server") }; if strings.Contains(server, "litespeed") { add("LiteSpeed", "", "server") }; if strings.Contains(server, "caddy") { add("Caddy", "", "server") }; if strings.Contains(server, "cloudflare") || headers.Get("cf-ray") != "" { add("Cloudflare", "", "cdn") }; if headers.Get("x-akamai-request-id") != "" { add("Akamai", "", "cdn") }; if headers.Get("x-fastly-request-id") != "" { add("Fastly", "", "cdn") }; if headers.Get("x-amz-cf-id") != "" { add("CloudFront", "", "cdn") }; if headers.Get("x-sucuri-id") != "" { add("Sucuri", "", "cdn") }; powered := strings.ToLower(headers.Get("X-Powered-By")); if strings.Contains(powered, "php") { add("PHP", extractVersion(headers.Get("X-Powered-By")), "language") }; if strings.Contains(powered, "express") { add("Node.js/Express", extractVersion(headers.Get("X-Powered-By")), "framework") }; if strings.Contains(powered, "asp.net") { add("ASP.NET", extractVersion(headers.Get("X-Powered-By")), "framework") }; if headers.Get("X-AspNet-Version") != "" { add("ASP.NET", headers.Get("X-AspNet-Version"), "framework") }; if headers.Get("X-Drupal-Cache") != "" { add("Drupal", "", "cms") }; if strings.Contains(source, "/sites/default/files/") { add("Drupal", "", "cms") }; if strings.Contains(source, "wp-content/") || strings.Contains(source, "wp-includes/") { add("WordPress", "", "cms") }; if strings.Contains(source, "/media/jui/") || strings.Contains(source, "com_content") { add("Joomla", "", "cms") }; if strings.Contains(strings.ToLower(source), "laravel") { add("Laravel", "", "framework") }; if strings.Contains(headers.Get("Set-Cookie"), "csrftoken") { add("Django", "", "framework") }; if strings.Contains(headers.Get("Set-Cookie"), "_rails_session") { add("Ruby on Rails", "", "framework") }; if strings.Contains(source, "__NEXT_DATA__") { add("Next.js", "", "framework") }; if strings.Contains(source, "__NUXT__") { add("Nuxt.js", "", "framework") }; if strings.Contains(source, "react-dom") || strings.Contains(source, "data-reactroot") { add("React", "", "framework") }; if strings.Contains(source, "vue.js") || strings.Contains(source, "data-v-") { add("Vue.js", "", "framework") }; if strings.Contains(source, "ng-version") { add("Angular", "", "framework") }; if strings.Contains(source, "jquery.min.js") || strings.Contains(source, "jquery.js") { add("jQuery", extractJQueryVersion(source), "library") }; if strings.Contains(source, "bootstrap.min.css") { add("Bootstrap", "", "library") }; if strings.Contains(source, "elementor") { add("Elementor", "", "plugin") }; genRe := regexp.MustCompile(`<meta[^>]+name=["']generator["'][^>]+content=["']([^"']+)["']`); if m := genRe.FindStringSubmatch(source); len(m) > 1 { content := m[1]; if strings.Contains(content, "WordPress") { vRe := regexp.MustCompile(`WordPress\s+([0-9.]+)`); if v := vRe.FindStringSubmatch(content); len(v) > 1 { add("WordPress", v[1], "cms") } } }; return techs }
func extractVersion(header string) string { re := regexp.MustCompile(`([0-9]+\.[0-9]+(\.[0-9]+)?)`); if m := re.FindStringSubmatch(header); len(m) > 1 { return m[1] }; return "" }
func extractJQueryVersion(source string) string { re := regexp.MustCompile(`jquery(?:\.min)?\.js\?ver=([0-9.]+)`); if m := re.FindStringSubmatch(source); len(m) > 1 { return m[1] }; return "" }
func printTech(techs []TechFingerprint) { if len(techs) == 0 { return }; printSection("⚙️  TECH FINGERPRINT"); for _, t := range techs { val := t.Name; if t.Version != "" { val += " " + t.Version }; printField(t.Category, val, colorCyan) }; printFooter() }
func checkDNSSecurity(target string) *DNSSecurity { u, _ := url.Parse(target); host := u.Hostname(); parts := strings.Split(host, "."); rootDomain := host; if len(parts) >= 2 { rootDomain = strings.Join(parts[len(parts)-2:], ".") }; dns := &DNSSecurity{}; if txts, err := net.LookupTXT(rootDomain); err == nil { for _, txt := range txts { if strings.HasPrefix(txt, "v=spf1") { dns.SPF = txt; break } } }; if dns.SPF == "" { dns.Issues = append(dns.Issues, "Tidak ada SPF record — email spoofing mungkin") } else if strings.Contains(dns.SPF, "+all") || strings.Contains(dns.SPF, "?all") { dns.Issues = append(dns.Issues, "SPF terlalu permisif") }; if txts, err := net.LookupTXT("_dmarc." + rootDomain); err == nil { for _, txt := range txts { if strings.HasPrefix(txt, "v=DMARC1") { dns.DMARC = txt; break } } }; if dns.DMARC == "" { dns.Issues = append(dns.Issues, "Tidak ada DMARC record") } else if !strings.Contains(dns.DMARC, "p=reject") && !strings.Contains(dns.DMARC, "p=quarantine") { dns.Issues = append(dns.Issues, "DMARC policy lemah") }; for _, sel := range []string{"default", "google", "mail", "k1"} { if txts, err := net.LookupTXT(sel + "._domainkey." + rootDomain); err == nil && len(txts) > 0 { dns.DKIM = sel + " selector ada"; break } }; if dns.DKIM == "" { dns.Issues = append(dns.Issues, "Tidak ada DKIM record (selector umum)") }; return dns }
func printDNS(dns *DNSSecurity) { if dns == nil { return }; printSection("📧 DNS SECURITY (SPF/DMARC/DKIM)"); if dns.SPF != "" { printField("SPF", dns.SPF, colorGreen) } else { printField("SPF", "MISSING", colorRed) }; if dns.DMARC != "" { printField("DMARC", dns.DMARC, colorGreen) } else { printField("DMARC", "MISSING", colorRed) }; if dns.DKIM != "" { printField("DKIM", dns.DKIM, colorGreen) } else { printField("DKIM", "MISSING", colorYellow) }; for _, issue := range dns.Issues { printField("Issue", issue, colorYellow) }; printFooter() }
func shannonEntropy(s string) float64 { if len(s) == 0 { return 0.0 }; counts := make(map[rune]int); for _, c := range s { counts[c]++ }; length := float64(len([]rune(s))); var ent float64; for _, count := range counts { p := float64(count) / length; ent -= p * math.Log2(p) }; return ent }
func isLikelySecretValue(value string) bool { v := strings.Trim(value, `"' `); if falsePositiveValues[strings.ToLower(v)] || len(v) < 12 { return false }; hasAlpha := regexp.MustCompile(`[a-zA-Z]`).MatchString(v); hasDigit := regexp.MustCompile(`[0-9]`).MatchString(v); return (hasAlpha && hasDigit) || shannonEntropy(v) >= 3.8 }
func maskSecret(value string, visible int) string { runes := []rune(value); if len(runes) <= visible { return strings.Repeat("*", len(runes)) }; return string(runes[:visible]) + strings.Repeat("*", len(runes)-visible) }
func extractSecretsFromText(text, sourceLabel string) []SecretFinding { findings := []SecretFinding{}; seen := make(map[string]bool); nameValRegex := regexp.MustCompile(`(?i)["']?(` + secretNamePattern.String()[4:len(secretNamePattern.String())-2] + `)["']?\s*[:=]\s*["']([^"'\s]{8,200})["']`); for _, m := range nameValRegex.FindAllStringSubmatch(text, -1) { if len(m) < 3 || !isLikelySecretValue(m[2]) { continue }; key := strings.ToLower(m[1]) + "|" + m[2]; if seen[key] { continue }; seen[key] = true; findings = append(findings, SecretFinding{Type: "named_variable", Name: m[1], ValueMasked: maskSecret(m[2], 6), RawValue: m[2], Source: sourceLabel}) }; for _, sp := range secretValuePatterns { for _, m := range sp.re.FindAllString(text, -1) { if seen[m] { continue }; seen[m] = true; findings = append(findings, SecretFinding{Type: "value_pattern", Name: sp.name, ValueMasked: maskSecret(m, 6), RawValue: m, Source: sourceLabel}) } }; return findings }
func scanSecretsInPage(client *http.Client, baseURL, source string, noJS bool, maxJS, timeout int) []SecretFinding { printSection("🔑 SECRET / API KEY / TOKEN"); var all []SecretFinding; all = append(all, extractSecretsFromText(source, "HTML")...); if !noJS { doc, err := goquery.NewDocumentFromReader(strings.NewReader(source)); if err == nil { doc.Find("script[src]").Each(func(i int, s *goquery.Selection) { if i >= maxJS { return }; src, _ := s.Attr("src"); if strings.HasPrefix(src, "data:") { return }; jsURL := resolveURL(baseURL, src); jsClient := newHTTPClient(timeout, true); jsText, _, _, _, _ := fetchSource(jsClient, jsURL); if jsText == "" || len(jsText) > 5_000_000 { return }; all = append(all, extractSecretsFromText(jsText, jsURL)...) }) } }; dedup := make(map[string]SecretFinding); for _, f := range all { key := strings.ToLower(f.Name) + "|" + f.RawValue; if _, ok := dedup[key]; !ok { dedup[key] = f } }; final := make([]SecretFinding, 0, len(dedup)); for _, f := range dedup { final = append(final, f) }; if len(final) == 0 { printField("Status", "Tidak ada secret terdeteksi", colorGreen); printFooter(); return final }; for _, f := range final { printField(f.Name, f.ValueMasked, colorRed) }; printFooter(); return final }
func resolveURL(base, ref string) string { bu, _ := url.Parse(base); ru, _ := url.Parse(ref); return bu.ResolveReference(ru).String() }
func normalizeURL(u string) string { u = strings.TrimSpace(u); if u == "" { return "" }; if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") { u = "https://" + u }; return u }
func analyzeStatic(source, baseURL string) *StaticFindings { f := &StaticFindings{HiddenInputs: []string{}, Forms: []string{}, JSVars: []string{}, PotentialSinks: []string{}, WPSignals: []string{}, PluginVersions: map[string][]string{}, ThemeVersions: map[string][]string{}}; doc, err := goquery.NewDocumentFromReader(strings.NewReader(source)); if err != nil { return f }; doc.Find(`input[type="hidden"]`).Each(func(i int, s *goquery.Selection) { if name, ok := s.Attr("name"); ok && name != "" { val, _ := s.Attr("value"); f.HiddenInputs = append(f.HiddenInputs, name+"="+val) } }); doc.Find("form").Each(func(i int, s *goquery.Selection) { action, _ := s.Attr("action"); action = resolveURL(baseURL, action); method, _ := s.Attr("method"); if method == "" { method = "GET" }; var fields []string; s.Find("input, select, textarea").Each(func(j int, inp *goquery.Selection) { if n, ok := inp.Attr("name"); ok && n != "" { fields = append(fields, n) } }); if len(fields) > 0 { f.Forms = append(f.Forms, fmt.Sprintf("%s %s -> %v", strings.ToUpper(method), action, fields)) } }); wpPatterns := []struct{ re *regexp.Regexp; name string }{{regexp.MustCompile(`/wp-content/`), "Folder /wp-content/"}, {regexp.MustCompile(`/wp-includes/`), "Folder /wp-includes/"}, {regexp.MustCompile(`wp-json`), "WP REST API"}, {regexp.MustCompile(`admin-ajax\.php`), "admin-ajax.php"}}; for _, wp := range wpPatterns { if wp.re.MatchString(source) { f.WPSignals = append(f.WPSignals, wp.name) } }; doc.Find(`meta[name="generator"]`).Each(func(i int, s *goquery.Selection) { if content, ok := s.Attr("content"); ok && content != "" { f.WPSignals = append(f.WPSignals, "generator: "+content); m := regexp.MustCompile(`WordPress\s+([0-9.]+)`).FindStringSubmatch(content); if len(m) > 1 { f.WPVersion = m[1] } } }); pluginRe := regexp.MustCompile(`/wp-content/plugins/([a-zA-Z0-9_\-]+)/[^"']*?\?ver=([0-9.]+)`); for _, m := range pluginRe.FindAllStringSubmatch(source, -1) { f.PluginVersions[m[1]] = append(f.PluginVersions[m[1]], m[2]) }; themeRe := regexp.MustCompile(`/wp-content/themes/([a-zA-Z0-9_\-]+)/[^"']*?\?ver=([0-9.]+)`); for _, m := range themeRe.FindAllStringSubmatch(source, -1) { f.ThemeVersions[m[1]] = append(f.ThemeVersions[m[1]], m[2]) }; return f }
func printStaticFindings(f *StaticFindings) { if len(f.HiddenInputs) > 0 { printSection("📝 HIDDEN INPUTS"); for _, i := range f.HiddenInputs { printField("Input", i, colorCyan) }; printFooter() }; if len(f.Forms) > 0 { printSection("📋 FORMS & PARAMETER"); for _, i := range f.Forms { printField("Form", i, colorCyan) }; printFooter() }; if len(f.WPSignals) > 0 { printSection("🌍 SINYAL WORDPRESS"); for _, i := range f.WPSignals { printField("Signal", i, colorWhite) }; printFooter() }; if len(f.PluginVersions) > 0 { printSection("🧩 PLUGIN VERSIONS"); keys := make([]string, 0, len(f.PluginVersions)); for k := range f.PluginVersions { keys = append(keys, k) }; sort.Strings(keys); for _, k := range keys { printField(k, strings.Join(f.PluginVersions[k], ", "), colorCyan) }; printFooter() } }
func extractJSEndpoints(client *http.Client, baseURL, source string, maxJS, timeout int) *JSEndpoints { doc, _ := goquery.NewDocumentFromReader(strings.NewReader(source)); scripts := doc.Find("script[src]"); if scripts.Length() > 0 { printSection(fmt.Sprintf("🕸️  JS FILES (Maks %d)", maxJS)) }; patterns := []*regexp.Regexp{regexp.MustCompile(`["'](/wp-json/[^"']+)["']`), regexp.MustCompile(`["']([^"']*admin-ajax\.php[^"']*)["']`), regexp.MustCompile(`["'](/api/[^"']+)["']`)}; ajaxDataRe := regexp.MustCompile(`data\s*:\s*\{([^}]{1,800})\}`); actionRe := regexp.MustCompile(`action\s*:\s*["']([^"']+)["']`); endpoints := map[string]bool{}; actions := map[string]bool{}; scripts.Each(func(i int, s *goquery.Selection) { if i >= maxJS { return }; src, ok := s.Attr("src"); if !ok || strings.HasPrefix(src, "data:") { return }; jsURL := resolveURL(baseURL, src); jsClient := newHTTPClient(timeout, true); text, _, _, _, _ := fetchSource(jsClient, jsURL); if text == "" { return }; for _, pat := range patterns { for _, m := range pat.FindAllStringSubmatch(text, -1) { endpoints[m[1]] = true } }; for _, block := range ajaxDataRe.FindAllStringSubmatch(text, -1) { for _, act := range actionRe.FindAllStringSubmatch(block[1], -1) { actions[act[1]] = true } } }); epList := make([]string, 0, len(endpoints)); for e := range endpoints { epList = append(epList, e) }; sort.Strings(epList); acList := make([]string, 0, len(actions)); for a := range actions { acList = append(acList, a) }; sort.Strings(acList); if len(epList) == 0 && len(acList) == 0 && scripts.Length() > 0 { printField("Status", "Tidak ada endpoint unik", colorGray); printFooter() } else { for _, e := range epList { printField("Endpoint", e, colorWhite) }; for _, a := range acList { printField("AJAX Action", a, colorWhite) }; if scripts.Length() > 0 { printFooter() } }; return &JSEndpoints{Endpoints: epList, Actions: acList} }
func checkWPUsers(client *http.Client, baseURL string, timeout int) []WPUser { printSection("👥 WP USER ENUMERATION"); usersURL := resolveURL(baseURL, "/wp-json/wp/v2/users"); c := newHTTPClient(timeout, true); text, _, status, _, _ := fetchSource(c, usersURL); if status == 401 || status == 403 { printField("Status", fmt.Sprintf("Diblokir (HTTP %d) — Bagus", status), colorGreen); printFooter(); return []WPUser{} }; if status != 200 || text == "" { printField("Status", fmt.Sprintf("HTTP %d — Tidak mengekspos user", status), colorGreen); printFooter(); return []WPUser{} }; var data []map[string]interface{}; if err := json.Unmarshal([]byte(text), &data); err != nil || len(data) == 0 { printField("Status", "Bukan list user valid", colorGray); printFooter(); return []WPUser{} }; printField("Status", fmt.Sprintf("%d user terekspos", len(data)), colorRed); out := []WPUser{}; limit := 10; if len(data) < limit { limit = len(data) }; for _, u := range data[:limit] { name, _ := u["name"].(string); slug, _ := u["slug"].(string); id := u["id"]; printField("User", fmt.Sprintf("id=%v, name=%q, slug=%q", id, name, slug), colorYellow); out = append(out, WPUser{ID: id, Name: name, Slug: slug}) }; printFooter(); return out }
func analyzeJWT(token string) *JWTInfo { parts := strings.Split(token, "."); if len(parts) != 3 { return nil }; headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0]); if err != nil { return nil }; var header map[string]interface{}; if err := json.Unmarshal(headerBytes, &header); err != nil { return nil }; payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1]); if err != nil { return nil }; var payload map[string]interface{}; if err := json.Unmarshal(payloadBytes, &payload); err != nil { return nil }; info := &JWTInfo{Raw: token}; if alg, ok := header["alg"].(string); ok { info.Alg = alg; if alg == "none" { info.IsNone = true; info.Warnings = append(info.Warnings, "alg: none (VULNERABLE — bypass signature)") } }; if typ, ok := header["typ"].(string); ok { info.Type = typ }; for k, v := range payload { valStr := fmt.Sprintf("%v", v); info.Claims = append(info.Claims, fmt.Sprintf("%s=%s", k, valStr)); if k == "exp" { if expFloat, ok := v.(float64); ok { expTime := time.Unix(int64(expFloat), 0); if expTime.Before(time.Now()) { info.IsExpired = true; info.Warnings = append(info.Warnings, "Token expired ("+expTime.Format("2006-01-02")+")") } } }; if k == "role" || k == "admin" { if strings.Contains(strings.ToLower(valStr), "admin") || valStr == "true" { info.Warnings = append(info.Warnings, "Klaim admin ditemukan ("+k+"="+valStr+")") } } }; return info }
func printJWT(j *JWTInfo) { if j == nil { return }; printSection("🎫 JWT ANALYSIS"); c := colorGreen; if j.IsNone { c = colorRed }; printField("Algorithm", j.Alg, c); printField("Type", j.Type, colorGray); if len(j.Claims) > 0 { limit := 5; if len(j.Claims) < limit { limit = len(j.Claims) }; for _, cl := range j.Claims[:limit] { printField("Claim", cl, colorCyan) } }; if len(j.Warnings) > 0 { for _, w := range j.Warnings { printField("⚠ Warning", w, colorRed) } } else { printField("Status", "Tidak ada anomali", colorGreen) }; printFooter() }
func discoverParameters(source string) []string { doc, err := goquery.NewDocumentFromReader(strings.NewReader(source)); if err != nil { return nil }; params := map[string]bool{}; doc.Find("input, select, textarea").Each(func(i int, s *goquery.Selection) { if name, ok := s.Attr("name"); ok && name != "" { params[name] = true } }); doc.Find("a[href]").Each(func(i int, s *goquery.Selection) { href, _ := s.Attr("href"); if u, err := url.Parse(href); err == nil { for k := range u.Query() { params[k] = true } } }); list := make([]string, 0, len(params)); for p := range params { list = append(list, p) }; return list }
func fuzzDiscoveredParams(client *http.Client, baseURL string, source string, timeout int) []string { params := discoverParameters(source); if len(params) == 0 { return nil }; printSection("🎯 PARAMETER DISCOVERY & FUZZING"); baseline, _, baselineStatus, _, _ := fetchSource(client, baseURL); baselineLen := len(baseline); found := []string{}; for _, param := range params { testURL := baseURL + "?" + param + "=test123"; resp, _, status, _, _ := fetchSource(client, testURL); if resp == "" { continue }; diffLen := abs(len(resp) - baselineLen); reflected := strings.Contains(resp, "test123"); statusChanged := status != baselineStatus; if diffLen > 50 || reflected || statusChanged { found = append(found, param); printField(param, fmt.Sprintf("len_diff=%d reflected=%v status=%d->%d", diffLen, reflected, baselineStatus, status), colorYellow) } }; if len(found) == 0 { printField("Status", "Tidak ada parameter form mencurigakan", colorGreen) }; printFooter(); return found }
func abs(n int) int { if n < 0 { return -n }; return n }
func checkWPVulnerabilities(plugin, version string) []Vulnerability { apiURL := fmt.Sprintf("https://www.wpvulnerability.com/api/plugin/%s", plugin); client := &http.Client{Timeout: 5 * time.Second}; resp, err := client.Get(apiURL); if err != nil || resp.StatusCode != 200 { return nil }; defer resp.Body.Close(); var data struct { Vulnerabilities []struct { ID string `json:"id"`; Title string `json:"title"`; Operator string `json:"operator"`; Version string `json:"version"`; Severity string `json:"severity"` } `json:"vulnerabilities"` }; if err := json.NewDecoder(resp.Body).Decode(&data); err != nil { return nil }; var found []Vulnerability; for _, v := range data.Vulnerabilities { if v.Operator == "<=" { v1, _ := strconv.ParseFloat(version, 64); v2, _ := strconv.ParseFloat(v.Version, 64); if v1 > 0 && v2 > 0 && v1 <= v2 { found = append(found, Vulnerability{ID: v.ID, Title: v.Title, Severity: v.Severity}) } } }; return found }
func computeScore(r *ScanResult) (int, []string) { score := 100; reasons := []string{}; deduct := func(points int, reason string) { score -= points; reasons = append(reasons, fmt.Sprintf("-%d %s", points, reason)) }; if r.Error != "" { return 0, []string{"gagal fetch"} }; if r.SSL != nil && !r.SSL.OK { deduct(10, "SSL invalid") }; for _, h := range r.Headers { if h.Status == "missing" { deduct(5, "header "+h.Name+" hilang") } }; if r.CORS != nil && r.CORS.Vulnerable { deduct(25, "CORS misconfig") }; for _, e := range r.Exposures { deduct(30, "file ekspos "+e.Path) }; if len(r.Secrets) > 0 { deduct(50, fmt.Sprintf("%d secret", len(r.Secrets))) }; if r.Takeover != nil && r.Takeover.Vulnerable { deduct(40, "takeover") }; if len(r.WPUsers) > 0 { deduct(15, "user enum") }; if len(r.WPVulns) > 0 { deduct(30, "WP CVE") }; if len(r.TechVulns) > 0 { deduct(30, "Tech CVE") }; if r.DNS != nil && len(r.DNS.Issues) > 0 { deduct(5*len(r.DNS.Issues), "DNS security issue") }; if score < 0 { score = 0 }; return score, reasons }

// ============================================================
// SCAN ONE
// ============================================================
func scanOne(target string, args *Args) *ScanResult {
	start := time.Now()
	r := &ScanResult{Target: target, Secrets: []SecretFinding{}, WPUsers: []WPUser{}, Fuzz: []string{}, Exposures: []ExposureFinding{}, Headers: []HeaderFinding{}, WPVulns: []string{}, TechStack: []TechFingerprint{}, JWTs: []*JWTInfo{}}
	if !args.Quiet { fmt.Printf("\n%s\n", colorize(colorBold+colorWhite, "SCANNING: "+target)) }
	r.SSL = diagnoseSSL(target, args.Timeout); if !args.Quiet { printSSLDiagnosis(r.SSL) }
	r.DNS = checkDNSSecurity(target); if !args.Quiet { printDNS(r.DNS) }
	client := newHTTPClient(args.Timeout, false)
	source, sslOK, status, finalURL, respHeaders := fetchSource(client, target)
	r.HTTPStatus = status; r.FinalURL = finalURL
	if source == "" { r.Error = "gagal fetch"; if !args.Quiet { printSection("❌ CONNECTION"); printField("Status", r.Error, colorRed); printFooter() }; r.DurationMS = time.Since(start).Milliseconds(); return r }
	r.Size = len(source)
	r.TechStack = fingerprintTech(source, respHeaders); if !args.Quiet { printTech(r.TechStack) }
	
	// NVD CVE Check
	if args.CVE {
		for _, t := range r.TechStack {
			if t.Version != "" && (t.Name == "PHP" || t.Name == "Nginx" || t.Name == "Apache") {
				cves := queryNVD(strings.ToLower(t.Name), t.Version)
				if len(cves) > 0 {
					r.TechVulns = append(r.TechVulns, cves...)
					if !args.Quiet { printNVDResults(t.Name, t.Version, cves) }
				}
			}
		}
	}

	r.Headers = auditHeaders(respHeaders); if !args.Quiet { printHeaders(r.Headers) }
	r.CORS = checkCORS(client, target); if !args.Quiet { printCORS(r.CORS) }
	r.Preflight = checkCORSPreflight(client, target); if !args.Quiet { printPreflight(r.Preflight) }
	r.WAF = detectWAF(client, target); if !args.Quiet { printWAF(r.WAF) }
	if args.Exposures { r.Exposures = checkExposures(client, target) }
	r.Takeover = checkTakeover(target, source); if !args.Quiet { printTakeover(r.Takeover) }
	r.Static = analyzeStatic(source, target); if !args.Quiet { printStaticFindings(r.Static) }
	if len(r.Static.PluginVersions) > 0 { if !args.Quiet { printSection("🧩 CVE WORDPRESS CHECKER") }; for plugin, vers := range r.Static.PluginVersions { for _, v := range vers { vulns := checkWPVulnerabilities(plugin, v); for _, vuln := range vulns { cveStr := fmt.Sprintf("[%s] %s v%s: %s", vuln.Severity, plugin, v, vuln.ID); r.WPVulns = append(r.WPVulns, cveStr); if !args.Quiet { printField("CVE", cveStr, colorRed) } } } }; if !args.Quiet { printFooter() } }
	if !args.NoJS { r.JS = extractJSEndpoints(client, target, source, args.MaxJS, args.Timeout) }
	r.Secrets = scanSecretsInPage(client, target, source, args.NoJS, args.MaxJS, args.Timeout)
	if r.Static != nil && len(r.Static.WPSignals) > 0 { r.WPUsers = checkWPUsers(client, target, args.Timeout) }
	for _, s := range r.Secrets { if s.Type == "value_pattern" && s.Name == "JWT Token" { if info := analyzeJWT(s.RawValue); info != nil { r.JWTs = append(r.JWTs, info); if !args.Quiet { printJWT(info) } } } }
	if args.Fuzz { r.Fuzz = fuzzDiscoveredParams(client, target, source, args.Timeout) }
	r.Score, r.ScoreReasons = computeScore(r); r.DurationMS = time.Since(start).Milliseconds()
	if !args.Quiet { printSection("✅ SCAN SUMMARY"); c := colorGreen; if r.Score < 50 { c = colorRed } else if r.Score < 80 { c = colorYellow }; printField("HTTP Status", fmt.Sprintf("%d", r.HTTPStatus), colorWhite); printField("SSL Chain", func() string { if sslOK { return "Valid" }; return "Invalid" }(), colorWhite); printField("Final Score", fmt.Sprintf("%d/100", r.Score), c); printFooter() }
	return r
}

// ============================================================
// HTML REPORT, WORKER POOL, MAIN
// ============================================================
func generateHTMLReport(results []*ScanResult, filename string) { f, err := os.Create(filename); if err != nil { return }; defer f.Close(); const tpl = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>WebScanner Report</title><style>body{font-family:'Consolas',monospace;background:#121212;color:#d4d4d4;padding:20px}h1,h2{color:#00d2ff}.target-card{background:#1e1e1e;border:1px solid #333;border-radius:6px;padding:15px;margin-bottom:25px}.target-header{font-size:1.2em;font-weight:bold;color:#fff;margin-bottom:15px;border-bottom:1px solid #444;padding-bottom:5px;display:flex;justify-content:space-between}.box{border-left:2px solid #555;padding-left:15px;margin-bottom:15px}.box-title{color:#00d2ff;font-weight:bold;margin-bottom:5px}.field{margin:2px 0}.label{color:#888;display:inline-block;width:200px}.value-ok{color:#50fa7b}.value-fail{color:#ff5555;font-weight:bold}.value-warn{color:#f1fa8c}table{border-collapse:collapse;width:100%;margin-top:20px;background:#1e1e1e}th,td{border:1px solid #333;padding:10px;text-align:left}th{background:#252526;color:#00d2ff}.critical{color:#ff5555;font-weight:bold}.warning{color:#f1fa8c}.normal{color:#50fa7b}</style></head><body><h1>🛡️ WebScanner Detailed Report</h1><p>Total Targets: {{len .}}</p><h2>Ringkasan</h2><table><tr><th>Target</th><th>Score</th><th>Status</th><th>SSL</th><th>Secrets</th><th>Exposures</th><th>CVE</th><th>WAF</th></tr>{{range .}}<tr><td>{{.Target}}</td><td class="{{if lt .Score 50}}critical{{else if lt .Score 80}}warning{{else}}normal{{end}}">{{.Score}}</td><td>{{if .Error}}<span class="critical">ERROR</span>{{else}}Scanned{{end}}</td><td>{{if .SSL}}{{if .SSL.OK}}<span class="normal">Valid</span>{{else}}<span class="critical">Invalid</span>{{end}}{{else}}-{{end}}</td><td>{{len .Secrets}}</td><td>{{len .Exposures}}</td><td>{{len .WPVulns}}</td><td>{{if .WAF}}{{if .WAF.Detected}}<span class="warning">{{.WAF.Name}}</span>{{else}}<span class="normal">None</span>{{end}}{{else}}-{{end}}</td></tr>{{end}}</table><hr style="border-color:#333;margin:40px 0"><h2>Detail per Target</h2>{{range .}}{{if not .Error}}<div class="target-card"><div class="target-header"><span>{{.Target}}</span><span class="{{if lt .Score 50}}critical{{else if lt .Score 80}}warning{{else}}normal{{end}}">Score: {{.Score}}/100</span></div>{{if .SSL}}<div class="box"><div class="box-title">┌── 🔒 SSL DIAGNOSIS</div><div class="field"><span class="label">│ Host</span>: {{.SSL.Host}}</div><div class="field"><span class="label">│ Status</span>: <span class="{{if .SSL.OK}}value-ok{{else}}value-fail{{end}}">{{if .SSL.OK}}OK{{else}}{{.SSL.Reason}}{{end}}</span></div></div>{{end}}{{if .TechStack}}<div class="box"><div class="box-title">┌── ⚙️ TECH FINGERPRINT</div>{{range .TechStack}}<div class="field"><span class="label">│ {{.Category}}</span>: {{.Name}} {{.Version}}</div>{{end}}</div>{{end}}{{if .TechVulns}}<div class="box"><div class="box-title">┌── 🧩 TECH CVE CHECKER</div>{{range .TechVulns}}<div class="field"><span class="label value-fail">│ {{.ID}}</span>: [{{.Severity}}] {{.Description}}</div>{{end}}</div>{{end}}{{if .DNS}}<div class="box"><div class="box-title">┌── 📧 DNS SECURITY</div><div class="field"><span class="label">│ SPF</span>: <span class="{{if .DNS.SPF}}value-ok{{else}}value-fail{{end}}">{{if .DNS.SPF}}{{.DNS.SPF}}{{else}}MISSING{{end}}</span></div><div class="field"><span class="label">│ DMARC</span>: <span class="{{if .DNS.DMARC}}value-ok{{else}}value-fail{{end}}">{{if .DNS.DMARC}}{{.DNS.DMARC}}{{else}}MISSING{{end}}</span></div>{{if .DNS.Issues}}{{range .DNS.Issues}}<div class="field"><span class="label value-warn">│ Issue</span>: {{.}}</div>{{end}}{{end}}</div>{{end}}{{if .Headers}}<div class="box"><div class="box-title">┌── 🛡️ SECURITY HEADERS</div>{{range .Headers}}<div class="field"><span class="label">│ [{{if eq .Status "ok"}} OK {{else if eq .Status "missing"}}FAIL{{else if eq .Status "weak"}}WEAK{{else}}INFO{{end}}]</span> {{.Name}} <span class="{{if eq .Status "missing"}}value-fail{{else if eq .Status "weak"}}value-warn{{else}}value-ok{{end}}">{{.Note}}</span></div>{{end}}</div>{{end}}{{if .WAF}}<div class="box"><div class="box-title">┌── 🛡️ WAF & RATE LIMIT</div><div class="field"><span class="label">│ WAF</span>: <span class="{{if .WAF.Detected}}value-warn{{else}}value-ok{{end}}">{{if .WAF.Detected}}{{.WAF.Name}}{{else}}Tidak terdeteksi{{end}}</span></div><div class="field"><span class="label">│ Rate Limited</span>: <span class="{{if .WAF.RateLimited}}value-warn{{else}}value-ok{{end}}">{{if .WAF.RateLimited}}YA{{else}}Tidak{{end}}</span></div>{{if .WAF.RateLimitNote}}<div class="field"><span class="label value-warn">│ Note</span>: {{.WAF.RateLimitNote}}</div>{{end}}</div>{{end}}{{if .CORS}}<div class="box"><div class="box-title">┌── 🌐 CORS CHECK</div><div class="field"><span class="label">│ Allow-Origin</span>: <span class="{{if .CORS.Vulnerable}}value-fail{{else}}value-ok{{end}}">{{if .CORS.AllowOrigin}}{{.CORS.AllowOrigin}}{{else}}Tidak ada{{end}}</span></div></div>{{end}}{{if .Preflight}}<div class="box"><div class="box-title">┌── 🌐 CORS PREFLIGHT</div><div class="field"><span class="label">│ Allow-Methods</span>: {{.Preflight.AllowMethods}}</div><div class="field"><span class="label">│ Max-Age</span>: {{.Preflight.MaxAge}}</div><div class="field"><span class="label">│ Vulnerable</span>: <span class="{{if .Preflight.Vulnerable}}value-fail{{else}}value-ok{{end}}">{{if .Preflight.Vulnerable}}YA{{else}}Tidak{{end}}</span></div></div>{{end}}{{if .Takeover}}<div class="box"><div class="box-title">┌── 🚨 SUBDOMAIN TAKEOVER</div><div class="field"><span class="label">│ Status</span>: <span class="{{if .Takeover.Vulnerable}}value-fail{{else}}value-warn{{end}}">{{if .Takeover.Vulnerable}}VULNERABLE ({{.Takeover.Confidence}}){{else}}Potential{{end}}</span></div><div class="field"><span class="label">│ Service</span>: {{.Takeover.Service}}</div><div class="field"><span class="label">│ CNAME</span>: {{.Takeover.CNAME}}</div></div>{{end}}{{if .Exposures}}<div class="box"><div class="box-title">┌── 📂 EXPOSURE FILES</div>{{range .Exposures}}<div class="field"><span class="label value-fail">│ {{.Category}}</span>: {{.Path}} ({{.Size}} bytes)</div>{{end}}</div>{{end}}{{if .Secrets}}<div class="box"><div class="box-title">┌── 🔑 SECRETS</div>{{range .Secrets}}<div class="field"><span class="label value-fail">│ {{.Name}}</span>: {{.ValueMasked}} <span style="color:#888">[{{.Source}}]</span></div>{{end}}</div>{{end}}{{if .JWTs}}{{range .JWTs}}<div class="box"><div class="box-title">┌── 🎫 JWT ANALYSIS</div><div class="field"><span class="label">│ Algorithm</span>: <span class="{{if .IsNone}}value-fail{{else}}value-ok{{end}}">{{.Alg}}</span></div><div class="field"><span class="label">│ Type</span>: {{.Type}}</div>{{range .Claims}}<div class="field"><span class="label">│ Claim</span>: {{.}}</div>{{end}}{{range .Warnings}}<div class="field"><span class="label value-fail">│ ⚠ Warning</span>: {{.}}</div>{{end}}</div>{{end}}{{end}}{{if .WPVulns}}<div class="box"><div class="box-title">┌── 🧩 WP CVEs</div>{{range .WPVulns}}<div class="field"><span class="label value-fail">│ CVE</span>: {{.}}</div>{{end}}</div>{{end}}</div>{{end}}{{end}}</body></html>`; t, err := template.New("report").Parse(tpl); if err != nil { fmt.Printf("[!] Template error: %v\n", err); return }; if err := t.Execute(f, results); err != nil { fmt.Printf("[!] Gagal tulis HTML: %v\n", err) } else { fmt.Printf("\n%s Laporan HTML disimpan ke: %s\n", colorize(colorGreen, "[+]"), filename) } }
func runWorkerPool(targets []string, args *Args) []*ScanResult { results := make([]*ScanResult, len(targets)); var wg sync.WaitGroup; var mu sync.Mutex; sem := make(chan struct{}, args.Concurrency); done := 0; total := len(targets); var bar *progressbar.ProgressBar; if args.Quiet { bar = progressbar.NewOptions(total, progressbar.OptionSetDescription("Scanning"), progressbar.OptionSetWriter(os.Stdout), progressbar.OptionShowCount(), progressbar.OptionShowIts(), progressbar.OptionSetItsString("host"), progressbar.OptionOnCompletion(func() { fmt.Println() }), progressbar.OptionSetTheme(progressbar.Theme{Saucer: "=", SaucerHead: ">", SaucerPadding: " ", BarStart: "[", BarEnd: "]"})) }; for i, target := range targets { wg.Add(1); go func(idx int, t string) { defer wg.Done(); sem <- struct{}{}; defer func() { <-sem }(); if !args.Quiet { fmt.Println("\n" + colorize(colorGray, strings.Repeat("━", 70))); fmt.Printf("%s %s\n", colorize(colorCyan+colorBold, fmt.Sprintf("TARGET %d/%d:", idx+1, total)), colorWhite+colorBold+t); fmt.Println(colorize(colorGray, strings.Repeat("━", 70))) }; var res *ScanResult; func() { defer func() { if rec := recover(); rec != nil { res = &ScanResult{Target: t, Error: fmt.Sprintf("panic: %v", rec), Secrets: []SecretFinding{}, WPUsers: []WPUser{}, Exposures: []ExposureFinding{}, Headers: []HeaderFinding{}} } }(); res = scanOne(t, args) }(); results[idx] = res; mu.Lock(); done++; currentDone := done; mu.Unlock(); if bar != nil { _ = bar.Add(1) }; if args.ProgressFile != "" && currentDone%5 == 0 { saveJSONPartial(results, args.ProgressFile) } }(i, target); if args.Sleep > 0 && i < len(targets)-1 { time.Sleep(time.Duration(args.Sleep * float64(time.Second))) } }; wg.Wait(); return results }
func printSummary(results []*ScanResult) { fmt.Println("\n" + colorize(colorGray, strings.Repeat("═", 135))); fmt.Println(colorize(colorBold+colorWhite, "  RINGKASAN PEMINDAIAN (ATTACK SURFACE)")); fmt.Println(colorize(colorGray, strings.Repeat("═", 135))); fmt.Printf("%-3s │ %-30s │ %-4s │ %-4s │ %-3s │ %-3s │ %-3s │ %-3s │ %-3s │ %-3s │ %s\n", "#", "Target", "HTTP", "SSL", "Sec", "Exp", "CVE", "TO", "DNS", "WAF", "Score/Status"); fmt.Println(colorize(colorGray, strings.Repeat("─", 135))); pad := func(s string, length int) string { if len(s) >= length { return s[:length] }; return s + strings.Repeat(" ", length-len(s)) }; for i, r := range results { if r == nil { continue }; http := pad("-", 4); if r.HTTPStatus > 0 { http = pad(fmt.Sprintf("%d", r.HTTPStatus), 4) }; sslStr := pad("-", 4); if r.SSL != nil { if r.SSL.OK { sslStr = colorize(colorGreen, " OK ") } else { sslStr = colorize(colorRed, "FAIL") } }; secStr := pad("-", 3); if len(r.Secrets) > 0 { secStr = colorize(colorRed, pad(fmt.Sprintf("%d", len(r.Secrets)), 3)) }; expStr := pad("-", 3); if len(r.Exposures) > 0 { expStr = colorize(colorRed, pad(fmt.Sprintf("%d", len(r.Exposures)), 3)) }; cveStr := pad("-", 3); if len(r.WPVulns) > 0 || len(r.TechVulns) > 0 { cveStr = colorize(colorRed, pad(fmt.Sprintf("%d", len(r.WPVulns)+len(r.TechVulns)), 3)) }; toStr := pad("-", 3); if r.Takeover != nil && r.Takeover.Vulnerable { toStr = colorize(colorRed, pad("!", 3)) } else if r.Takeover != nil { toStr = colorize(colorYellow, pad("?", 3)) }; dnsStr := pad("-", 3); if r.DNS != nil && len(r.DNS.Issues) > 0 { dnsStr = colorize(colorYellow, pad(fmt.Sprintf("%d", len(r.DNS.Issues)), 3)) }; wafStr := pad("-", 3); if r.WAF != nil && r.WAF.Detected { wafStr = colorize(colorYellow, pad("!", 3)) } else if r.WAF != nil { wafStr = colorize(colorGreen, pad("ok", 3)) }; scoreStr := pad("-", 4); statusStr := ""; if r.Error == "" { c := colorGreen; if r.Score < 50 { c = colorRed; statusStr = colorize(colorRed, "KRITIS") } else if r.Score < 80 { c = colorYellow; statusStr = colorize(colorYellow, "WARNING") } else { statusStr = colorize(colorGreen, "NORMAL") }; scoreStr = colorize(c, pad(fmt.Sprintf("%d", r.Score), 4)) } else { statusStr = colorize(colorRed, "ERROR"); scoreStr = colorize(colorRed, pad("0", 4)) }; target := pad(r.Target, 30); fmt.Printf("%-3d │ %s │ %s │ %s │ %s │ %s │ %s │ %s │ %s │ %s │ %s %s\n", i+1, target, http, sslStr, secStr, expStr, cveStr, toStr, dnsStr, wafStr, scoreStr, statusStr) }; fmt.Println(colorize(colorGray, strings.Repeat("─", 135))) }
type Args struct { URL, List, JSON, ProgressFile, OutputHTML string; Stdin, Fuzz, NoJS, Quiet, Exposures, OnlyInter, Resume, NoColor, CVE bool; MaxJS, Timeout, Concurrency int; Sleep float64 }
func parseArgs() *Args { a := &Args{}; flag.StringVar(&a.URL, "url", "", "URL target tunggal"); flag.StringVar(&a.List, "list", "", "File daftar URL"); flag.BoolVar(&a.Stdin, "stdin", false, "Baca daftar URL dari stdin"); flag.BoolVar(&a.Fuzz, "fuzz", false, "Aktifkan parameter discovery (fuzzing)"); flag.BoolVar(&a.NoJS, "no-js", false, "Lewati ekstraksi JS"); flag.IntVar(&a.MaxJS, "max-js", 15, "Batas jumlah file JS"); flag.IntVar(&a.Timeout, "timeout", 15, "Timeout per request (s)"); flag.BoolVar(&a.Quiet, "quiet", false, "Hanya progress bar + ringkasan"); flag.StringVar(&a.JSON, "json", "", "Simpan hasil ke JSON"); flag.Float64Var(&a.Sleep, "sleep", 0.5, "Jeda antar target (s)"); flag.IntVar(&a.Concurrency, "concurrency", 3, "Worker paralel"); flag.BoolVar(&a.Exposures, "exposures", true, "Cek common exposure files"); flag.StringVar(&a.ProgressFile, "progress-file", "", "Simpan partial JSON"); flag.BoolVar(&a.OnlyInter, "only-interesting", false, "Hanya tampilkan host skor < 70"); flag.BoolVar(&a.Resume, "resume", false, "Resume dari progress-file"); flag.BoolVar(&a.NoColor, "no-color", false, "Matikan output berwarna"); flag.StringVar(&a.OutputHTML, "html", "", "Simpan laporan HTML"); flag.BoolVar(&a.CVE, "cve", false, "Aktifkan NVD CVE check (butuh NVD_API_KEY)"); flag.Parse(); return a }
func loadTargets(args *Args) ([]string, bool) { targets := []string{}; if args.List != "" { data, err := os.ReadFile(args.List); if err != nil { fmt.Printf("[!] File tidak ditemukan: %s\n", args.List); os.Exit(1) }; for _, line := range strings.Split(string(data), "\n") { line = strings.TrimSpace(line); if line != "" && !strings.HasPrefix(line, "#") { targets = append(targets, normalizeURL(line)) } }; return targets, false }; if args.Stdin || (!isTerminal() && args.URL == "") { data, _ := io.ReadAll(os.Stdin); for _, line := range strings.Split(string(data), "\n") { line = strings.TrimSpace(line); if line != "" && !strings.HasPrefix(line, "#") { targets = append(targets, normalizeURL(line)) } }; if len(targets) > 0 { return targets, false } }; if args.URL != "" { return []string{normalizeURL(args.URL)}, true }; fmt.Print("Masukkan URL target: "); var input string; fmt.Scanln(&input); if input != "" { return []string{normalizeURL(input)}, true }; return []string{}, true }
func isTerminal() bool { fi, _ := os.Stdin.Stat(); return (fi.Mode() & os.ModeCharDevice) != 0 }
func saveJSON(results []*ScanResult, path string) { data, _ := json.MarshalIndent(results, "", "  "); os.WriteFile(path, data, 0644); fmt.Printf("\n[i] Hasil JSON disimpan ke: %s\n", path) }
func saveJSONPartial(results []*ScanResult, path string) { data, _ := json.MarshalIndent(results, "", "  "); os.WriteFile(path+".tmp", data, 0644); os.Rename(path+".tmp", path) }
func loadProgress(path string) map[string]bool { done := map[string]bool{}; data, err := os.ReadFile(path); if err != nil { return done }; var results []*ScanResult; json.Unmarshal(data, &results); for _, r := range results { if r != nil && r.Target != "" && r.Error == "" { done[r.Target] = true } }; return done }
func filterResume(targets []string, done map[string]bool) []string { out := []string{}; for _, t := range targets { if !done[t] { out = append(out, t) } }; return out }

func main() {
	args := parseArgs()
	if args.NoColor { os.Setenv("NO_COLOR", "1") }
	printBanner()
	targets, singleMode := loadTargets(args)
	if len(targets) == 0 { fmt.Println("[!] Tidak ada target."); os.Exit(1) }
	if args.Resume && args.ProgressFile != "" { done := loadProgress(args.ProgressFile); if len(done) > 0 { fmt.Printf("[i] Resume: %d host dilewati.\n", len(done)); targets = filterResume(targets, done); if len(targets) == 0 { fmt.Println("[i] Selesai."); return } } }
	concurrency := args.Concurrency; if singleMode { concurrency = 1 }; args.Concurrency = concurrency
	modeStr := "sekuensial"; if singleMode { modeStr = "tunggal" }
	fmt.Printf("=== Memindai %d target (%s, concurrency=%d) ===\n\n", len(targets), modeStr, concurrency)
	var results []*ScanResult
	if singleMode { results = []*ScanResult{scanOne(targets[0], args)} } else { results = runWorkerPool(targets, args) }
	display := results
	if args.OnlyInter { display = []*ScanResult{}; for _, r := range results { if r != nil && r.Error == "" && r.Score < 70 { display = append(display, r) } }; if len(display) == 0 { fmt.Println("\n[i] Tidak ada host skor < 70.") } }
	if !singleMode || len(results) > 1 { printSummary(display) }
	if args.JSON != "" { saveJSON(results, args.JSON) }
	if args.ProgressFile != "" { saveJSONPartial(results, args.ProgressFile) }
	if args.OutputHTML != "" { generateHTMLReport(results, args.OutputHTML) }
	fmt.Println("\n=== Selesai ===")
}