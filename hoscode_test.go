package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func quietNotes(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := notesOut
	notesOut = &buf
	t.Cleanup(func() { notesOut = prev })
	return &buf
}

func TestAppDirMovesTheOldFolderOnceAndLinksIt(t *testing.T) {
	notes := quietNotes(t)
	base := t.TempDir()
	old := filepath.Join(base, ".px0")
	if err := os.MkdirAll(filepath.Join(old, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "settings.json"), []byte(`{"workbench.colorTheme":"hos"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "sessions", "a.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := appDir(base, ".hos-code", ".px0")
	if dir != filepath.Join(base, ".hos-code") {
		t.Fatalf("dir = %s", dir)
	}
	for _, rel := range []string{"settings.json", "sessions/a.json"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("%s did not move: %v", rel, err)
		}
		if _, err := os.Stat(filepath.Join(old, rel)); err != nil {
			t.Fatalf("%s is not reachable through the old path: %v", rel, err)
		}
	}
	if fi, err := os.Lstat(old); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("old path should be a link, got %v %v", fi, err)
	}
	if !strings.Contains(notes.String(), "moved") {
		t.Fatalf("no note printed: %q", notes.String())
	}
	if again := appDir(base, ".hos-code", ".px0"); again != dir {
		t.Fatalf("second call = %s", again)
	}
}

func TestAppDirLeavesAnExistingNewFolderAlone(t *testing.T) {
	quietNotes(t)
	base := t.TempDir()
	for _, d := range []string{"px0", "hos-code"} {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := appDir(base, "hos-code", "px0"); got != filepath.Join(base, "hos-code") {
		t.Fatalf("got %s", got)
	}
	if fi, err := os.Lstat(filepath.Join(base, "px0")); err != nil || !fi.IsDir() {
		t.Fatalf("old folder should be untouched, got %v %v", fi, err)
	}
}

func TestAppDirUsesALinkedOldFolderInPlace(t *testing.T) {
	notes := quietNotes(t)
	base := t.TempDir()
	real := filepath.Join(base, "dotfiles", "px0")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "settings.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(base, ".px0")); err != nil {
		t.Fatal(err)
	}
	got := appDir(base, ".hos-code", ".px0")
	if _, err := os.Stat(filepath.Join(got, "settings.json")); err != nil {
		t.Fatalf("data behind the link not found via %s: %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(base, ".hos-code")); err == nil {
		t.Fatalf("a linked old folder should not be moved")
	}
	if !strings.Contains(notes.String(), "is a link") {
		t.Fatalf("no note printed: %q", notes.String())
	}
}

func TestAppDirNotesWhenBothFoldersExist(t *testing.T) {
	notes := quietNotes(t)
	base := t.TempDir()
	for _, d := range []string{".px0", ".hos-code"} {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, d, "settings.json"), []byte(d), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	appDir(base, ".hos-code", ".px0")
	if !strings.Contains(notes.String(), "both") {
		t.Fatalf("no note printed: %q", notes.String())
	}
	if b, err := os.ReadFile(filepath.Join(base, ".px0", "settings.json")); err != nil || string(b) != ".px0" {
		t.Fatalf("old data must stay, got %q %v", b, err)
	}
}

func TestSettingsPathMigratesXDGConfig(t *testing.T) {
	quietNotes(t)
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if err := os.MkdirAll(filepath.Join(cfg, "px0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "px0", "settings.json"), []byte(`{"explorer.autoReveal":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readSettings()
	if got.ExplorerAutoReveal == nil || *got.ExplorerAutoReveal {
		t.Fatalf("settings lost in migration: %+v", got)
	}
	if settingsPath() != filepath.Join(cfg, "hos-code", "settings.json") {
		t.Fatalf("settingsPath = %s", settingsPath())
	}
}

func TestGetenvCompatPrefersNewNameAndNotesOldOnce(t *testing.T) {
	notes := quietNotes(t)
	envNoted.Delete("OLD_NAME_FOR_TEST")
	env := map[string]string{"OLD_NAME_FOR_TEST": "old"}
	get := func(k string) string { return env[k] }
	if v := getenvCompat(get, "NEW_NAME_FOR_TEST", "OLD_NAME_FOR_TEST"); v != "old" {
		t.Fatalf("fallback = %q", v)
	}
	getenvCompat(get, "NEW_NAME_FOR_TEST", "OLD_NAME_FOR_TEST")
	if n := strings.Count(notes.String(), "deprecated"); n != 1 {
		t.Fatalf("want one deprecation note, got %d: %q", n, notes.String())
	}
	env["NEW_NAME_FOR_TEST"] = "new"
	if v := getenvCompat(get, "NEW_NAME_FOR_TEST", "OLD_NAME_FOR_TEST"); v != "new" {
		t.Fatalf("new name should win, got %q", v)
	}
}

func TestLoadHosLinkReadsNewTokenName(t *testing.T) {
	quietNotes(t)
	env := map[string]string{"HOS_URL": "http://127.0.0.1:4000", "HOS_CODE_TOKEN": "new", "HOS_PX0_TOKEN": "old"}
	got := loadHosLink(func(k string) string { return env[k] }, "")
	if got == nil || got.token != "new" {
		t.Fatalf("HOS_CODE_TOKEN should win, got %+v", got)
	}
}

func TestLoginGateOffLoopbackOrWithAToken(t *testing.T) {
	quietNotes(t)
	none := func(string) string { return "" }
	for _, host := range []string{"127.0.0.1", "::1", "localhost"} {
		if g, src := loginGateFor(host, 7777, "/", none); g != nil || src != loginOff {
			t.Fatalf("%s: loopback should need no login", host)
		}
	}
	chosen := map[string]string{loginTokenEnv: "from-hos"}
	for _, host := range []string{"127.0.0.1", "::1", "localhost"} {
		if g, src := loginGateFor(host, 7777, "/", func(k string) string { return chosen[k] }); g == nil || src != loginFromEnv || g.token != "from-hos" {
			t.Fatalf("%s: a token from hos must keep the login on loopback, got %+v %v", host, g, src)
		}
	}
	g, src := loginGateFor("100.64.0.5", 7777, "/", none)
	if g == nil || src != loginGenerated || len(g.token) < 32 {
		t.Fatalf("tailnet bind without a token should generate one, got %+v %v", g, src)
	}
	env := map[string]string{loginTokenEnv: "chosen"}
	g, src = loginGateFor("0.0.0.0", 7777, "/", func(k string) string { return env[k] })
	if g == nil || src != loginFromEnv || g.token != "chosen" {
		t.Fatalf("env token should be used, got %+v %v", g, src)
	}
	oldHosEnv := map[string]string{"HOS_URL": "http://127.0.0.1:9", "HOS_PX0_TOKEN": "bridge"}
	if g, src := loginGateFor("100.64.0.5", 7777, "/", func(k string) string { return oldHosEnv[k] }); g == nil || src != loginGenerated {
		t.Fatalf("only HOS_PX0_TOKEN set must still require a login, got %+v %v", g, src)
	}
}

func TestLoginTokenIsNotPassedToChildProcesses(t *testing.T) {
	t.Setenv(loginTokenEnv, "child-must-not-see")
	g, src := loginGateFromEnv("100.64.0.5", 7777, "/")
	if g == nil || src != loginFromEnv || g.token != "child-must-not-see" {
		t.Fatalf("the env token should still be used, got %+v %v", g, src)
	}
	if _, ok := os.LookupEnv(loginTokenEnv); ok {
		t.Fatal("the login token should be removed from the environment after it is read")
	}
	out, err := exec.Command("env").Output()
	if err != nil {
		t.Skip("env not available")
	}
	if strings.Contains(string(out), "child-must-not-see") {
		t.Fatal("a child process saw the login token")
	}
	if g, src := loginGateFromEnv("100.64.0.5", 7777, "/"); g == nil || src != loginGenerated {
		t.Fatalf("a generated token should not be written back to the environment, got %+v %v", g, src)
	}
	if _, ok := os.LookupEnv(loginTokenEnv); ok {
		t.Fatal("a generated token should not be written to the environment")
	}
}

func TestVerboseRequestLogRedactsLoginSecrets(t *testing.T) {
	cases := map[string]string{
		"/?path=a.go&token=abc123":        "/?path=a.go&token=REDACTED",
		"/rev/?token=abc123":              "/rev/?token=REDACTED",
		"/rev/view/k/deadbeef/doc/a.html": "/rev/view/k/REDACTED/doc/a.html",
		"/view/k/deadbeef":                "/view/k/REDACTED",
		"/api/files?path=x":               "/api/files?path=x",
		"/":                               "/",
	}
	for in, want := range cases {
		if got := redactLoginURI(in); got != want {
			t.Fatalf("redactLoginURI(%q) = %q, want %q", in, got, want)
		}
	}
	if got := redactLoginURI("/?token=abc123;x"); strings.Contains(got, "abc123") {
		t.Fatalf("an unparsable query must not leak the token, got %q", got)
	}
}

func TestLoginGateRefusesThenAdmitsByTokenCookieOrBearer(t *testing.T) {
	s, _ := newTestServer(t)
	g, _ := loginGateFor("100.64.0.5", 7777, "/", func(k string) string {
		if k == loginTokenEnv {
			return "secret-token"
		}
		return ""
	})
	s.login = g

	serve := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		return rec
	}

	if rec := serve(httptest.NewRequest(http.MethodGet, "/api/meta", nil)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("api without login = %d", rec.Code)
	}
	page := httptest.NewRequest(http.MethodGet, "/?path=main.go", nil)
	page.Header.Set("Accept", "text/html")
	rec := serve(page)
	body, _ := io.ReadAll(rec.Body)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(string(body), `name="token"`) || !strings.Contains(string(body), `value="main.go"`) {
		t.Fatalf("page without login = %d %s", rec.Code, body)
	}
	if rec := serve(httptest.NewRequest(http.MethodGet, "/static/favicon.svg?token=wrong", nil)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d", rec.Code)
	}
	post := httptest.NewRequest(http.MethodPost, "/api/git/push?token=secret-token", nil)
	if rec := serve(post); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a token in the query must not admit a POST, got %d", rec.Code)
	}

	rec = serve(httptest.NewRequest(http.MethodGet, "/?path=main.go&token=secret-token", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/?path=main.go" {
		t.Fatalf("token login = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = serve(httptest.NewRequest(http.MethodGet, "//evil.example/x?token=secret-token", nil))
	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || loc != "/evil.example/x" {
		t.Fatalf("token login must redirect on-site, got %d %q", rec.Code, loc)
	}
	rec = serve(httptest.NewRequest(http.MethodGet, "/?path=main.go&token=secret-token", nil))
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].Value == "secret-token" {
		t.Fatalf("cookie = %+v", cookies)
	}

	withCookie := httptest.NewRequest(http.MethodGet, "/api/meta", nil)
	withCookie.AddCookie(cookies[0])
	if rec := serve(withCookie); rec.Code != http.StatusOK {
		t.Fatalf("cookie login = %d", rec.Code)
	}
	bearer := httptest.NewRequest(http.MethodGet, "/api/meta", nil)
	bearer.Header.Set("Authorization", "Bearer secret-token")
	if rec := serve(bearer); rec.Code != http.StatusOK {
		t.Fatalf("bearer login = %d", rec.Code)
	}
}

func TestIndexServesOnlyLocalAssets(t *testing.T) {
	s, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	for _, banned := range []string{"px0.ai", "fonts.googleapis.com", "fonts.gstatic.com", "cdn.jsdelivr.net", "https://"} {
		if banned == "https://" {
			body = strings.ReplaceAll(body, "https://github.com/Harinder441/hos-code", "")
		}
		if strings.Contains(body, banned) {
			t.Fatalf("index.html still references %s", banned)
		}
	}
	for _, asset := range []string{"/static/favicon.svg", "/static/vendor/mermaid-12.0.0.min.js"} {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, asset, nil))
		if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
			t.Fatalf("%s = %d (%d bytes)", asset, rec.Code, rec.Body.Len())
		}
	}
}

func TestViewUnderLoginLoadsRelativeAssetsWithoutTheCookie(t *testing.T) {
	s, root := newTestServer(t)
	for rel, body := range map[string]string{
		"docs/page.html":   `<link rel="stylesheet" href="style.css"><img src="img/dot.png">`,
		"docs/style.css":   "body{}",
		"docs/img/dot.png": "png",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g, _ := loginGateFor("100.64.0.5", 7777, "/", func(k string) string {
		if k == loginTokenEnv {
			return "secret-token"
		}
		return ""
	})
	s.login = g
	srv := httptest.NewServer(s)
	defer srv.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	do := func(method, target string, cookie *http.Cookie) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+target, nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}

	login := do(http.MethodGet, "/?token=secret-token", nil)
	cookie := login.Cookies()[0]
	opened := do(http.MethodGet, "/view/docs/page.html", cookie)
	page := opened.Header.Get("Location")
	if opened.StatusCode != http.StatusSeeOther || page != "/view/k/"+g.view+"/docs/page.html" || opened.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("view with cookie = %d %q", opened.StatusCode, page)
	}
	if strings.Contains(page, g.value) || strings.Contains(page, g.token) {
		t.Fatalf("the view key must not be the login cookie or token: %q", page)
	}
	base, _ := url.Parse(page)
	for _, rel := range []string{"", "style.css", "img/dot.png"} {
		target := base.ResolveReference(&url.URL{Path: rel}).Path
		if rel == "" {
			target = page
		}
		res := do(http.MethodGet, target, nil)
		if res.StatusCode != http.StatusOK || res.Header.Get("Referrer-Policy") != "no-referrer" || !strings.HasPrefix(res.Header.Get("Content-Security-Policy"), "sandbox") {
			t.Fatalf("%s without the cookie = %d %q", target, res.StatusCode, res.Header.Get("Content-Security-Policy"))
		}
	}
	for _, target := range []string{
		"/view/docs/style.css",
		"/view/k/" + strings.Repeat("0", 64) + "/docs/style.css",
		"/view/k/" + g.view + "/../../api/meta",
		"/view/k/" + g.view + "/%2e%2e/%2e%2e/api/meta",
		"/api/meta",
	} {
		if res := do(http.MethodGet, target, nil); res.StatusCode == http.StatusOK || res.StatusCode == http.StatusSeeOther {
			t.Fatalf("%s without the cookie = %d", target, res.StatusCode)
		}
	}
	if res := do(http.MethodPost, page, nil); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a POST with the view key = %d", res.StatusCode)
	}

	s.login = nil
	if res := do(http.MethodGet, "/view/docs/page.html", nil); res.StatusCode != http.StatusOK {
		t.Fatalf("loopback view = %d", res.StatusCode)
	}
}
