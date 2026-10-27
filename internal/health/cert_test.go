package health

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckReadsCertificateExpiry(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	// The test server's certificate isn't trusted, so check it as an
	// "insecure" LAN service; the end date is read either way.
	h := Check(context.Background(), Target{ID: "x", URL: srv.URL, Insecure: true})
	if h.Status != "up" || h.CertExpires == nil || !h.CertExpires.Equal(srv.Certificate().NotAfter) {
		t.Fatalf("health = %+v, want up with the cert's end %v", h, srv.Certificate().NotAfter)
	}

	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer plain.Close()
	if h := Check(context.Background(), Target{ID: "y", URL: plain.URL}); h.CertExpires != nil {
		t.Errorf("plain http has no certificate: %+v", h)
	}
}

func TestCheckReportsExpiredCertificate(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ended := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    ended.Add(-90 * 24 * time.Hour),
		NotAfter:     ended,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	srv.StartTLS()
	defer srv.Close()

	h := Check(context.Background(), Target{ID: "z", URL: srv.URL})
	if h.Status != "down" || h.Reason != "certificate expired" || h.CertExpires == nil || !h.CertExpires.Equal(ended) {
		t.Errorf("health = %+v, want down, certificate expired, ended %v", h, ended)
	}
}
