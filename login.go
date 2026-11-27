package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
)

const loginTokenEnv = "HOS_CODE_LOGIN_TOKEN"

type loginSource int

const (
	loginOff loginSource = iota
	loginFromEnv
	loginGenerated
)

type loginGate struct {
	token  string
	cookie string
	value  string
	path   string
	view   string
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func loginGateFor(host string, port int, basePath string, getenv func(string) string) (*loginGate, loginSource) {
	token := strings.TrimSpace(getenv(loginTokenEnv))
	if token == "" && isLoopbackHost(host) {
		return nil, loginOff
	}
	src := loginFromEnv
	if token == "" {
		b := make([]byte, 24)
		if _, err := rand.Read(b); err != nil {
			panic(err)
		}
		token = hex.EncodeToString(b)
		src = loginGenerated
	}
	return &loginGate{
		token:  token,
		cookie: "hos_code_login_" + strconv.Itoa(port),
		value:  derive(token, appName+" login"),
		path:   cleanBasePath(basePath),
		view:   derive(token, appName+" view"),
	}, src
}

// Read once, then dropped from the environment so git, language servers, installers and the browser opener never inherit it.
func loginGateFromEnv(host string, port int, basePath string) (*loginGate, loginSource) {
	g, src := loginGateFor(host, port, basePath, withFileEnv(os.Getenv))
	_ = os.Unsetenv(loginTokenEnv)
	_ = os.Unsetenv(loginTokenEnv + "_FILE")
	return g, src
}

func redactLoginURI(uri string) string {
	p, q, hasQuery := strings.Cut(uri, "?")
	if before, rest, ok := strings.Cut(p, "/view/k/"); ok {
		if _, tail, ok := strings.Cut(rest, "/"); ok {
			p = before + "/view/k/REDACTED/" + tail
		} else {
			p = before + "/view/k/REDACTED"
		}
	}
	if !hasQuery {
		return p
	}
	vals, err := url.ParseQuery(q)
	if err != nil {
		return p + "?REDACTED"
	}
	if _, ok := vals["token"]; ok {
		vals.Set("token", "REDACTED")
		q = vals.Encode()
	}
	return p + "?" + q
}

func derive(token, purpose string) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(purpose))
	return hex.EncodeToString(mac.Sum(nil))
}

// A viewed HTML file has an opaque origin, so its images and CSS arrive without the login cookie; the key in its path admits them.
func (g *loginGate) viewPrefix() string {
	return g.path + "view/k/" + g.view + "/"
}

func (g *loginGate) admitsView(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	rest, ok := strings.CutPrefix(r.URL.Path, g.path+"view/k/")
	if !ok {
		return false
	}
	key, _, ok := strings.Cut(rest, "/")
	if !ok || subtle.ConstantTimeCompare([]byte(key), []byte(g.view)) != 1 {
		return false
	}
	return strings.HasPrefix(path.Clean(r.URL.Path)+"/", g.viewPrefix())
}

func (g *loginGate) matches(token string) bool {
	return subtle.ConstantTimeCompare([]byte(token), []byte(g.token)) == 1
}

func (g *loginGate) admit(w http.ResponseWriter, r *http.Request) bool {
	if g == nil {
		return true
	}
	if t, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && g.matches(strings.TrimSpace(t)) {
		return true
	}
	if c, err := r.Cookie(g.cookie); err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(g.value)) == 1 {
		return true
	}
	if g.admitsView(r) {
		return true
	}
	q := r.URL.Query()
	if t := q.Get("token"); t != "" && g.matches(t) && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		http.SetCookie(w, &http.Cookie{Name: g.cookie, Value: g.value, Path: g.path, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 3600})
		q.Del("token")
		clean := path.Clean("/" + r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/") && clean != "/" {
			clean += "/"
		}
		next := url.URL{Path: clean, RawQuery: q.Encode()}
		http.Redirect(w, r, next.RequestURI(), http.StatusSeeOther)
		return false
	}
	if r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "/api/") && strings.Contains(r.Header.Get("Accept"), "text/html") {
		g.loginPage(w, r, q.Get("token") != "")
		return false
	}
	fail(w, http.StatusUnauthorized, "login required: open the link hos gave you, or add ?token=<"+loginTokenEnv+">")
	return false
}

var loginPageTmpl = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>hos-code · sign in</title>
<style>
:root{color-scheme:dark}
body{margin:0;min-height:100vh;display:grid;place-items:center;background:#0f1115;color:#e6e8eb;font:14px/1.5 ui-sans-serif,system-ui,sans-serif}
form{box-sizing:border-box;width:min(360px,90vw);display:grid;gap:12px;padding:24px;border:1px solid #2a2f37;border-radius:10px;background:#161a20}
h1{margin:0;font:600 18px ui-monospace,SFMono-Regular,Menlo,monospace}
p{margin:0;color:#9aa3ad}
input{padding:8px 10px;border-radius:6px;border:1px solid #2a2f37;background:#0f1115;color:inherit;font:inherit}
button{padding:8px 10px;border:0;border-radius:6px;background:#7dd3fc;color:#0f1115;font-weight:600;cursor:pointer}
.err{color:#fca5a5}
</style></head><body>
<form method="get">
<h1>hos-code</h1>
<p>This viewer listens on a network address, so it asks for its login token. Open the link hos gave you, or paste the token printed when hos-code started.</p>
{{if .Wrong}}<p class="err">That token is not right.</p>{{end}}
{{range $k, $v := .Keep}}<input type="hidden" name="{{$k}}" value="{{$v}}">{{end}}
<input name="token" type="password" autocomplete="current-password" placeholder="login token" autofocus required>
<button type="submit">Sign in</button>
</form></body></html>`))

func (g *loginGate) loginPage(w http.ResponseWriter, r *http.Request, wrong bool) {
	keep := map[string]string{}
	for k, v := range r.URL.Query() {
		if k != "token" && len(v) > 0 {
			keep[k] = v[0]
		}
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = loginPageTmpl.Execute(w, map[string]any{"Keep": keep, "Wrong": wrong})
}

func withLoginToken(raw, token string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String()
}
