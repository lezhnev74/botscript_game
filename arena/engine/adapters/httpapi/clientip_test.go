package httpapi

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/lezhnev/arena/engine/app"
)

var dockerNet = proxies{netip.MustParsePrefix("172.16.0.0/12")}

func requestFrom(peer string, forwarded ...string) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = peer + ":40000"
	for _, f := range forwarded {
		r.Header.Add("X-Forwarded-For", f)
	}
	return r
}

func TestClientHostBelievesOnlyTrustedProxies(t *testing.T) {
	cases := []struct {
		name string
		p    proxies
		r    *http.Request
		want string
	}{
		{"no proxies configured", nil, requestFrom("172.20.0.2", "9.9.9.9"), "172.20.0.2"},
		{"untrusted peer", dockerNet, requestFrom("8.8.8.8", "9.9.9.9"), "8.8.8.8"},
		{"trusted peer", dockerNet, requestFrom("172.20.0.2", "9.9.9.9"), "9.9.9.9"},
		{"client-sent entries left of the real one", dockerNet, requestFrom("172.20.0.2", "1.1.1.1, 9.9.9.9"), "9.9.9.9"},
		{"several headers and proxy hops", dockerNet, requestFrom("172.20.0.2", "1.1.1.1", "9.9.9.9, 172.20.0.5"), "9.9.9.9"},
		{"blank entries skipped", dockerNet, requestFrom("172.20.0.2", "9.9.9.9, ,"), "9.9.9.9"},
		{"trusted peer without the header", dockerNet, requestFrom("172.20.0.2"), "172.20.0.2"},
		{"only proxies in the header", dockerNet, requestFrom("172.20.0.2", "172.20.0.9"), "172.20.0.9"},
		{"ipv6 client", dockerNet, requestFrom("172.20.0.2", "2001:db8::1"), "2001:db8::1"},
		{"garbage entry is not trusted", dockerNet, requestFrom("172.20.0.2", "nonsense"), "nonsense"},
	}
	for _, c := range cases {
		if got := c.p.clientHost(c.r); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestClientHostOfAnAddressWithoutPort(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "pipe"
	if got := dockerNet.clientHost(r); got != "pipe" {
		t.Error(got)
	}
}

func TestHTTPSComesFromTLSOrATrustedProxy(t *testing.T) {
	direct := requestFrom("8.8.8.8")
	direct.TLS = &tls.ConnectionState{}
	spoofed := requestFrom("8.8.8.8")
	spoofed.Header.Set("X-Forwarded-Proto", "https")
	viaNginx := requestFrom("172.20.0.2")
	viaNginx.Header.Set("X-Forwarded-Proto", "HTTPS")
	plainViaNginx := requestFrom("172.20.0.2")
	plainViaNginx.Header.Set("X-Forwarded-Proto", "http")
	for r, want := range map[*http.Request]bool{direct: true, spoofed: false, viaNginx: true, plainViaNginx: false} {
		if got := dockerNet.https(r); got != want {
			t.Errorf("%s %v: %v, want %v", r.RemoteAddr, r.Header, got, want)
		}
	}
}

func TestCheckRateLimitIsPerForwardedClientBehindATrustedProxy(t *testing.T) {
	h := New(nil, Options{Check: checkHandlerFunc, Now: (&testClock{t: time.Unix(1000, 0)}).now, TrustedProxies: dockerNet})
	post := func(client string) int {
		req := httptest.NewRequest("POST", "/api/check", strings.NewReader("x"))
		req.RemoteAddr = "172.20.0.2:9"
		req.Header.Set("X-Forwarded-For", client)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w.Code
	}
	for range checkLimit {
		post("1.1.1.1")
	}
	if c := post("1.1.1.1"); c != 429 {
		t.Errorf("over the limit: %d, want 429", c)
	}
	if c := post("2.2.2.2"); c != 200 {
		t.Errorf("other client behind the same proxy: %d", c)
	}
}

func checkHandlerFunc(string) (app.CheckResult, error) {
	return app.CheckResult{Trial: &app.Trial{}}, nil
}
