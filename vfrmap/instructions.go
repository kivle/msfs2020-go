package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
)

var certificateTemplate = template.Must(template.New("certPage").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>vfrmap TLS certificate</title>
  <style>
    body { font-family: "Segoe UI", Tahoma, sans-serif; margin: 0; padding: 24px; background: #0f172a; color: #e2e8f0; }
    header { margin-bottom: 24px; }
    h1 { font-size: 28px; margin: 0 0 8px; }
    h2 { margin-top: 24px; }
    p { max-width: 920px; line-height: 1.5; }
    a { color: #38bdf8; }
    .downloads { display: flex; gap: 12px; flex-wrap: wrap; margin: 16px 0 8px; }
    .card { background: #1f2937; border: 1px solid #334155; padding: 12px 14px; border-radius: 8px; min-width: 220px; }
    .steps { background: #111827; border: 1px solid #1f2937; border-radius: 8px; padding: 12px 14px; margin-top: 12px; max-width: 960px; }
    ol { padding-left: 20px; }
    code { background: #0b1222; padding: 2px 6px; border-radius: 4px; }
  </style>
</head>
<body>
  <header>
    <h1>Secure WebSocket setup (wss://)</h1>
    <p>vfrmap generates a self-signed TLS certificate automatically. Install the certificate on devices you use to view the map so <code>wss://</code> connections are trusted.</p>
    <p>You can reach this page over plain HTTP (<code>{{.HTTPURL}}</code>) to download the certificate if your browser blocks HTTPS initially.</p>
    <div class="downloads">
      <div class="card">
        <strong>Certificate (PEM)</strong><br>
        <a href="{{.PEMLink}}">{{.PEMLink}}</a>
      </div>
      <div class="card">
        <strong>Certificate (DER)</strong><br>
        <a href="{{.DERLink}}">{{.DERLink}}</a>
      </div>
      <div class="card">
        <strong>Secure WebSocket (recommended)</strong><br>
        <code>{{.WSSURL}}</code>
      </div>
      <div class="card">
        <strong>Fallback WebSocket (no TLS)</strong><br>
        <code>{{.WSURL}}</code>
      </div>
    </div>

    <div class="card" style="margin-top:12px; max-width: 960px;">
      <strong>Copy-paste endpoints</strong>
      <p>Use these exact URLs in external apps. HTTPS/WSS are preferred after you install the certificate.</p>
      <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap:10px;">
        <div>
          <em>HTTPS / WSS</em>
          <ul style="padding-left:18px; margin:4px 0;">
          {{range .WSSList}}
            <li><code>{{.}}</code></li>
          {{end}}
          </ul>
        </div>
        <div>
          <em>HTTP / WS</em>
          <ul style="padding-left:18px; margin:4px 0;">
          {{range .WSList}}
            <li><code>{{.}}</code></li>
          {{end}}
          </ul>
        </div>
      </div>
    </div>
  </header>

  <section class="steps">
    <h2>Windows</h2>
    <ol>
      <li>Download the PEM file and open it.</li>
      <li>Click <em>Install Certificate</em> > <em>Local Machine</em> > place in <em>Trusted Root Certification Authorities</em>.</li>
      <li>Restart the browser and reconnect to <code>{{.WSSURL}}</code>.</li>
    </ol>
  </section>

  <section class="steps">
    <h2>macOS</h2>
    <ol>
      <li>Download the PEM file and double-click to open in Keychain Access.</li>
      <li>Add it to <em>System</em> keychain and set <em>When using this certificate</em> to <em>Always Trust</em>.</li>
      <li>Restart the browser and reconnect to <code>{{.WSSURL}}</code>.</li>
    </ol>
  </section>

  <section class="steps">
    <h2>iOS / iPadOS</h2>
    <ol>
      <li>On the device, open Safari to <code>http://{your-pc-ip}:{{.HTTPPort}}/cert.pem</code> (plain HTTP, no trust needed). Tap <em>Allow</em> to download the profile.</li>
      <li>If you prefer offline transfer, you can still AirDrop/email the PEM and tap it to install.</li>
      <li>Go to Settings > Profile Downloaded, install it, then Settings > General > About > Certificate Trust Settings and enable full trust.</li>
      <li>Open the map page using <code>{{.WSSURL}}</code>.</li>
    </ol>
  </section>

  <section class="steps">
    <h2>Linux</h2>
    <ol>
      <li>Download the PEM file.</li>
      <li>For system-wide trust on most distros: copy to <code>/usr/local/share/ca-certificates/vfrmap.pem</code> and run <code>sudo update-ca-certificates</code>, then restart your browser.</li>
      <li>If your distro uses <code>update-ca-trust</code>, place it under <code>/etc/pki/ca-trust/source/anchors/</code> and run <code>sudo update-ca-trust extract</code>.</li>
    </ol>
  </section>

  <section class="steps">
    <h2>Android</h2>
    <ol>
      <li>Download the DER file.</li>
      <li>Open it and choose to install a <em>VPN and apps</em> or <em>CA</em> certificate (wording varies by vendor).</li>
      <li>Confirm and reconnect to <code>{{.WSSURL}}</code>.</li>
    </ol>
  </section>

  <section class="steps">
    <h2>Firefox (desktop)</h2>
    <ol>
      <li>Download the PEM file.</li>
      <li>Open Firefox > Settings > Privacy & Security > Certificates > View Certificates.</li>
      <li>On the Authorities tab, click <em>Import</em>, select the PEM, and enable <em>Trust this CA to identify websites</em>.</li>
      <li>Reconnect to <code>{{.WSSURL}}</code>.</li>
    </ol>
  </section>
</body>
</html>`))

func certificateDownloadHandler(assets *TLSAssets, format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			data        []byte
			filename    string
			contentType string
		)

		switch format {
		case "pem":
			data = assets.CertPEM
			filename = "vfrmap-cert.pem"
			contentType = "application/x-pem-file"
		case "der":
			data = assets.CertDER
			filename = "vfrmap-cert.der"
			contentType = "application/pkix-cert"
		default:
			http.Error(w, "unknown certificate format", http.StatusNotFound)
			return
		}

		if len(data) == 0 {
			http.Error(w, "certificate not available", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Type", contentType)
		w.Write(data)
	}
}

func certificateInfoHandler(assets *TLSAssets, httpListen, httpsListen string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		hostOnly := hostWithoutPort(r.Host)
		httpPort := portFromAddr(httpListen)
		httpsPort := portFromAddr(httpsListen)

		httpHostPorts := collectHostPorts(httpListen, hostOnly)
		httpsHostPorts := collectHostPorts(httpsListen, hostOnly)

		httpHostPort := hostWithPort(hostOnly, httpPort)
		httpsHostPort := hostWithPort(hostOnly, httpsPort)

		wsList := makeURLs("ws", httpHostPorts, "/ws")
		wssList := makeURLs("wss", httpsHostPorts, "/ws")

		data := struct {
			PEMLink  string
			DERLink  string
			WSURL    string
			WSSURL   string
			HTTPURL  string
			HTTPSURL string
			HTTPPort string
			WSList   []string
			WSSList  []string
		}{
			PEMLink:  "/cert.pem",
			DERLink:  "/cert.der",
			WSURL:    (&url.URL{Scheme: "ws", Host: httpHostPort, Path: "/ws"}).String(),
			WSSURL:   (&url.URL{Scheme: "wss", Host: httpsHostPort, Path: "/ws"}).String(),
			HTTPURL:  (&url.URL{Scheme: "http", Host: httpHostPort, Path: "/"}).String(),
			HTTPSURL: (&url.URL{Scheme: "https", Host: httpsHostPort, Path: "/"}).String(),
			HTTPPort: httpPort,
			WSList:   wsList,
			WSSList:  wssList,
		}

		_ = certificateTemplate.Execute(w, data)
	}
}

func statusHandler(httpListen, httpsListen string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Header().Set("Content-Type", "application/json")

		hostOnly := hostWithoutPort(r.Host)
		httpPort := portFromAddr(httpListen)
		httpsPort := portFromAddr(httpsListen)

		httpHostPort := hostWithPort(hostOnly, httpPort)
		httpsHostPort := hostWithPort(hostOnly, httpsPort)

		jsonPayload := map[string]interface{}{
			"status":    "ok",
			"message":   "no embedded map UI; connect over websockets",
			"ws_path":   "/ws",
			"ws_url":    (&url.URL{Scheme: "ws", Host: httpHostPort, Path: "/ws"}).String(),
			"wss_url":   (&url.URL{Scheme: "wss", Host: httpsHostPort, Path: "/ws"}).String(),
			"cert_pem":  "/cert.pem",
			"cert_der":  "/cert.der",
			"cert_page": "/",
		}
		buf, _ := json.Marshal(jsonPayload)
		w.Write(buf)
	}
}

func portFromAddr(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return port
}

func hostWithoutPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err == nil {
		return host
	}
	return hostport
}

func hostWithPort(host, port string) string {
	if port == "" {
		return host
	}
	return net.JoinHostPort(host, port)
}

func collectHostPorts(listenAddr, requestHost string) []string {
	hosts := []string{}

	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		host = listenAddr
	}

	requestBase := hostWithoutPort(requestHost)
	if requestBase != "" {
		hosts = append(hosts, hostWithPort(requestBase, port))
	}

	if host != "" && host != "0.0.0.0" && host != "::" {
		hosts = append(hosts, hostWithPort(host, port))
	} else {
		addrs, err := net.InterfaceAddrs()
		if err == nil {
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() {
					continue
				}
				if ip.To4() != nil {
					hosts = append(hosts, hostWithPort(ip.String(), port))
				}
			}
		}
		hosts = append(hosts, hostWithPort("127.0.0.1", port))
	}

	return dedupe(hosts)
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func makeURLs(scheme string, hosts []string, path string) []string {
	urls := make([]string, 0, len(hosts))
	for _, h := range hosts {
		urls = append(urls, (&url.URL{Scheme: scheme, Host: h, Path: path}).String())
	}
	return urls
}
