package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const renewServerCertificateBefore = 30 * 24 * time.Hour

type tlsMaterial struct {
	caCert      string
	serverCert  string
	serverKey   string
	fingerprint string
	hosts       []string
}

type tlsPaths struct {
	caCert     string
	caKey      string
	serverCert string
	serverKey  string
}

func parseTLSHosts(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func ensureTLSMaterial(dir string, configuredHosts []string, listenAddr string, now time.Time) (tlsMaterial, error) {
	hosts, err := normalizeTLSHosts(configuredHosts, listenAddr)
	if err != nil {
		return tlsMaterial{}, err
	}
	if dir == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return tlsMaterial{}, fmt.Errorf("find user config directory: %w", err)
		}
		dir = filepath.Join(configDir, "mcp-1panel", "tls")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return tlsMaterial{}, fmt.Errorf("create TLS directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return tlsMaterial{}, fmt.Errorf("secure TLS directory: %w", err)
	}

	paths := tlsPaths{
		caCert:     filepath.Join(dir, "ca.crt"),
		caKey:      filepath.Join(dir, "ca.key"),
		serverCert: filepath.Join(dir, "server.crt"),
		serverKey:  filepath.Join(dir, "server.key"),
	}
	caCertExists, err := regularFileExists(paths.caCert)
	if err != nil {
		return tlsMaterial{}, err
	}
	caKeyExists, err := regularFileExists(paths.caKey)
	if err != nil {
		return tlsMaterial{}, err
	}
	serverCertExists, err := regularFileExists(paths.serverCert)
	if err != nil {
		return tlsMaterial{}, err
	}
	serverKeyExists, err := regularFileExists(paths.serverKey)
	if err != nil {
		return tlsMaterial{}, err
	}

	if caCertExists != caKeyExists {
		return tlsMaterial{}, errors.New("local CA certificate and private key must either both exist or both be absent")
	}
	if serverCertExists != serverKeyExists {
		return tlsMaterial{}, errors.New("server certificate and private key must either both exist or both be absent")
	}
	if !caCertExists && (serverCertExists || serverKeyExists) {
		return tlsMaterial{}, errors.New("server certificate exists without its local CA; refusing to replace the trusted CA")
	}

	var caCert *x509.Certificate
	var caKey *ecdsa.PrivateKey
	if !caCertExists {
		caCert, caKey, err = generateLocalCA(paths, now)
		if err != nil {
			return tlsMaterial{}, err
		}
	} else {
		caCert, caKey, err = loadLocalCA(paths, now)
		if err != nil {
			return tlsMaterial{}, err
		}
	}

	if !serverCertExists {
		if err := generateServerCertificate(paths, caCert, caKey, hosts, now); err != nil {
			return tlsMaterial{}, err
		}
	} else {
		renew, err := serverCertificateNeedsRenewal(paths, caCert, hosts, now)
		if err != nil {
			return tlsMaterial{}, err
		}
		if renew {
			if err := generateServerCertificate(paths, caCert, caKey, hosts, now); err != nil {
				return tlsMaterial{}, err
			}
		}
	}

	return tlsMaterial{
		caCert:      paths.caCert,
		serverCert:  paths.serverCert,
		serverKey:   paths.serverKey,
		fingerprint: certificateFingerprint(caCert),
		hosts:       hosts,
	}, nil
}

func normalizeTLSHosts(configured []string, listenAddr string) ([]string, error) {
	listenHost, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil, fmt.Errorf("parse TLS listen address %q: %w", listenAddr, err)
	}
	listenHost = strings.Trim(listenHost, "[]")
	if len(configured) == 0 {
		ip := net.ParseIP(listenHost)
		if listenHost == "" || (ip != nil && ip.IsUnspecified()) {
			return nil, errors.New("-tls-hosts is required when listening on a wildcard address")
		}
		configured = []string{listenHost}
	}

	hosts := make([]string, 0, len(configured))
	seen := make(map[string]struct{}, len(configured))
	for _, value := range configured {
		host := strings.Trim(strings.TrimSpace(value), "[]")
		if host == "" {
			return nil, errors.New("TLS host must not be empty")
		}
		if ip := net.ParseIP(host); ip != nil {
			host = ip.String()
		} else {
			host = strings.TrimSuffix(strings.ToLower(host), ".")
			if !validDNSName(host) {
				return nil, fmt.Errorf("invalid TLS DNS name %q", value)
			}
		}
		if _, exists := seen[host]; exists {
			continue
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}
	return hosts, nil
}

func validDNSName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}

func regularFileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect TLS file %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("TLS path %s is not a regular file", path)
	}
	return true, nil
}

func generateLocalCA(paths tlsPaths, now time.Time) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate local CA key: %w", err)
	}
	serial, err := randomSerialNumber()
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "mcp-1panel Local CA", Organization: []string{"mcp-1panel"}},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create local CA certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse generated local CA certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal local CA key: %w", err)
	}
	if err := atomicWriteFile(paths.caKey, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		return nil, nil, err
	}
	if err := atomicWriteFile(paths.caCert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644); err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

func loadLocalCA(paths tlsPaths, now time.Time) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	if err := securePrivateFile(paths.caKey); err != nil {
		return nil, nil, err
	}
	cert, err := readCertificate(paths.caCert)
	if err != nil {
		return nil, nil, fmt.Errorf("load local CA certificate: %w", err)
	}
	key, err := readECPrivateKey(paths.caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("load local CA key: %w", err)
	}
	publicKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok || !publicKey.Equal(&key.PublicKey) {
		return nil, nil, errors.New("local CA certificate and private key do not match")
	}
	if !cert.IsCA || cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		return nil, nil, errors.New("local CA certificate is not allowed to sign certificates")
	}
	if now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
		return nil, nil, errors.New("local CA certificate is not currently valid; rotate it explicitly and update client trust")
	}
	return cert, key, nil
}

func generateServerCertificate(paths tlsPaths, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, hosts []string, now time.Time) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate server certificate key: %w", err)
	}
	serial, err := randomSerialNumber()
	if err != nil {
		return err
	}
	notAfter := now.AddDate(1, 0, 0)
	if notAfter.After(caCert.NotAfter) {
		notAfter = caCert.NotAfter
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: hosts[0], Organization: []string{"mcp-1panel"}},
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create server certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal server certificate key: %w", err)
	}
	if err := atomicWriteFile(paths.serverKey, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		return err
	}
	if err := atomicWriteFile(paths.serverCert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644); err != nil {
		return err
	}
	return nil
}

func serverCertificateNeedsRenewal(paths tlsPaths, caCert *x509.Certificate, hosts []string, now time.Time) (bool, error) {
	if err := securePrivateFile(paths.serverKey); err != nil {
		return false, err
	}
	pair, err := tls.LoadX509KeyPair(paths.serverCert, paths.serverKey)
	if err != nil {
		return false, fmt.Errorf("load server certificate and key: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return false, errors.New("server certificate file contains no certificate")
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return false, fmt.Errorf("parse server certificate: %w", err)
	}
	if err := cert.CheckSignatureFrom(caCert); err != nil {
		return false, fmt.Errorf("server certificate was not signed by the local CA: %w", err)
	}
	serverAuth := false
	for _, usage := range cert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			serverAuth = true
			break
		}
	}
	if cert.IsCA || !serverAuth || cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return true, nil
	}
	for _, host := range hosts {
		if err := cert.VerifyHostname(host); err != nil {
			return true, nil
		}
	}
	return now.Before(cert.NotBefore) || !now.Add(renewServerCertificateBefore).Before(cert.NotAfter), nil
}

func securePrivateFile(path string) error {
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("secure private key %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect private key %s: %w", path, err)
	}
	if info.Mode().Perm() != 0600 {
		return fmt.Errorf("private key %s must have mode 0600", path)
	}
	return nil
}

func readCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("invalid certificate PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}

func readECPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "EC PRIVATE KEY" {
		return nil, errors.New("invalid EC private key PEM")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func randomSerialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial number: %w", err)
	}
	if serial.Sign() == 0 {
		serial.SetInt64(1)
	}
	return serial, nil
}

func certificateFingerprint(cert *x509.Certificate) string {
	digest := sha256.Sum256(cert.Raw)
	parts := make([]string, len(digest))
	for index, value := range digest {
		parts[index] = fmt.Sprintf("%02X", value)
	}
	return strings.Join(parts, ":")
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".tls-*")
	if err != nil {
		return fmt.Errorf("create temporary TLS file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return fmt.Errorf("set TLS file permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write TLS file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync TLS file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close TLS file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("install TLS file %s: %w", path, err)
	}
	return nil
}
