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
	"strconv"
	"strings"
)

const loginTokenEnv = "HOS_CODE_LOGIN_TOKEN"

type loginSource int

const (
	loginOff loginSource = iota
	loginFromEnv
	loginGenerated
	loginLegacyHos
)

type loginGate struct {
	token  string
	cookie string
	value  string
	path   string
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Launchers from before the rename (run as px0, or an hos passing only HOS_PX0_TOKEN) cannot add a login token to their links yet.
func loginGateFor(host string, port int, basePath string, getenv func(string) string, invokedAs string) (*loginGate, loginSource) {
	if isLoopbackHost(host) {
		return nil, loginOff
	}
	token := strings.TrimSpace(getenv(loginTokenEnv))
	src := loginFromEnv
	if token == "" {
		if strings.TrimSuffix(invokedAs, ".exe") == legacyName || (getenv("HOS_PX0_TOKEN") != "" && getenv("HOS_CODE_TOKEN") == "") {
			return nil, loginLegacyHos
		}
		b := make([]byte, 24)
		if _, err := rand.Read(b); err != nil {
			panic(err)
		}
		token = hex.EncodeToString(b)
		src = loginGenerated
	}
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(appName + " login"))
	return &loginGate{
		token:  token,
		cookie: "hos_code_login_" + strconv.Itoa(port),
		value:  hex.EncodeToString(mac.Sum(nil)),
		path:   cleanBasePath(basePath),
	}, src
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
	q := r.URL.Query()
	if t := q.Get("token"); t != "" && g.matches(t) && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		http.SetCookie(w, &http.Cookie{Name: g.cookie, Value: g.value, Path: g.path, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 3600})
		q.Del("token")
		next := url.URL{Path: r.URL.Path, RawQuery: q.Encode()}
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
