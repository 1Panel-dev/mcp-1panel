package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureTLSMaterial(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	material, err := ensureTLSMaterial(dir, []string{"localhost", "127.0.0.1", "localhost"}, "127.0.0.1:8000", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(material.hosts) != 2 || material.fingerprint == "" {
		t.Fatalf("unexpected TLS material: %#v", material)
	}

	for _, name := range []string{"ca.crt", "ca.key", "server.crt", "server.key"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"ca.key", "server.key"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("%s mode = %o, want 600", name, info.Mode().Perm())
		}
	}

	serverCert := readCertificateForTest(t, material.serverCert)
	if err := serverCert.VerifyHostname("localhost"); err != nil {
		t.Fatal(err)
	}
	if err := serverCert.VerifyHostname("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	caCert := readCertificateForTest(t, material.caCert)
	if err := serverCert.CheckSignatureFrom(caCert); err != nil {
		t.Fatal(err)
	}

	caBefore, err := os.ReadFile(material.caCert)
	if err != nil {
		t.Fatal(err)
	}
	serverBefore, err := os.ReadFile(material.serverCert)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ensureTLSMaterial(dir, []string{"127.0.0.1", "localhost"}, "127.0.0.1:8000", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	caAfter, _ := os.ReadFile(second.caCert)
	serverAfter, _ := os.ReadFile(second.serverCert)
	if !bytes.Equal(caBefore, caAfter) || !bytes.Equal(serverBefore, serverAfter) {
		t.Fatal("valid certificates were unexpectedly replaced")
	}

	renewed, err := ensureTLSMaterial(dir, []string{"127.0.0.1", "localhost"}, "127.0.0.1:8000", now.AddDate(0, 11, 15))
	if err != nil {
		t.Fatal(err)
	}
	renewedCA, _ := os.ReadFile(renewed.caCert)
	renewedServer, _ := os.ReadFile(renewed.serverCert)
	if !bytes.Equal(caBefore, renewedCA) {
		t.Fatal("server renewal replaced the local CA")
	}
	if bytes.Equal(serverBefore, renewedServer) {
		t.Fatal("expiring server certificate was not renewed")
	}
}

func TestEnsureTLSMaterialRejectsPartialFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	material, err := ensureTLSMaterial(dir, []string{"127.0.0.1"}, "127.0.0.1:8000", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(material.serverKey); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureTLSMaterial(dir, []string{"127.0.0.1"}, "127.0.0.1:8000", now); err == nil {
		t.Fatal("partial server certificate pair was accepted")
	}
	if _, err := ensureTLSMaterial(t.TempDir(), nil, ":8000", now); err == nil {
		t.Fatal("wildcard listener without tls-hosts was accepted")
	}
}

func TestGeneratedTLSCertificateHandshake(t *testing.T) {
	material, err := ensureTLSMaterial(t.TempDir(), []string{"127.0.0.1"}, "127.0.0.1:8000", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.LoadX509KeyPair(material.serverCert, material.serverKey)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()

	if _, err := http.Get(server.URL); err == nil {
		t.Fatal("untrusted local CA was accepted")
	}
	caPEM, err := os.ReadFile(material.caCert)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("failed to trust generated local CA")
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs:    roots,
		MinVersion: tls.VersionTLS12,
	}}}
	defer client.CloseIdleConnections()
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
}

func readCertificateForTest(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	certificate, err := readCertificate(path)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}
