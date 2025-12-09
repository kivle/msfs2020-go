package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type TLSAssets struct {
	CertPath string
	KeyPath  string
	CertPEM  []byte
	CertDER  []byte
}

func ensureTLSAssets(listenAddr string) (*TLSAssets, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate executable: %w", err)
	}
	exeDir := filepath.Dir(exe)

	certPath := filepath.Join(exeDir, "simconnect-ws-cert.pem")
	keyPath := filepath.Join(exeDir, "simconnect-ws-key.pem")

	certPEM, keyPEM, certDER, err := loadExistingCert(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	if certPEM != nil && keyPEM != nil {
		return &TLSAssets{
			CertPath: certPath,
			KeyPath:  keyPath,
			CertPEM:  certPEM,
			CertDER:  certDER,
		}, nil
	}

	return generateSelfSigned(certPath, keyPath, listenAddr)
}

func loadExistingCert(certPath, keyPath string) ([]byte, []byte, []byte, error) {
	certPEM, certErr := ioutil.ReadFile(certPath)
	keyPEM, keyErr := ioutil.ReadFile(keyPath)

	switch {
	case certErr == nil && keyErr == nil:
		return certPEM, keyPEM, parseDER(certPEM), nil
	case os.IsNotExist(certErr) || os.IsNotExist(keyErr):
		return nil, nil, nil, nil
	case certErr != nil:
		return nil, nil, nil, fmt.Errorf("read certificate file: %w", certErr)
	case keyErr != nil:
		return nil, nil, nil, fmt.Errorf("read key file: %w", keyErr)
	default:
		return nil, nil, nil, nil
	}
}

func generateSelfSigned(certPath, keyPath, listenAddr string) (*TLSAssets, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		serialNumber = big.NewInt(time.Now().UnixNano())
	}

	dnsNames, ipAddrs := sanForListen(listenAddr)
	tmpl := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "simconnect-ws",
			Organization: []string{"simconnect-ws"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(5, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ipAddrs,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	certPEM := new(bytes.Buffer)
	if err := pem.Encode(certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return nil, fmt.Errorf("encode certificate: %w", err)
	}

	keyPEM := new(bytes.Buffer)
	if err := pem.Encode(keyPEM, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}); err != nil {
		return nil, fmt.Errorf("encode key: %w", err)
	}

	if err := ioutil.WriteFile(certPath, certPEM.Bytes(), 0600); err != nil {
		return nil, fmt.Errorf("write certificate file: %w", err)
	}
	if err := ioutil.WriteFile(keyPath, keyPEM.Bytes(), 0600); err != nil {
		return nil, fmt.Errorf("write key file: %w", err)
	}

	return &TLSAssets{
		CertPath: certPath,
		KeyPath:  keyPath,
		CertPEM:  certPEM.Bytes(),
		CertDER:  certDER,
	}, nil
}

func sanForListen(listenAddr string) ([]string, []net.IP) {
	dnsNames := []string{"localhost"}
	ipAddrs := []net.IP{net.ParseIP("127.0.0.1")}

	host, _, err := net.SplitHostPort(listenAddr)
	if err == nil && host != "" && host != "0.0.0.0" && host != "::" {
		if ip := net.ParseIP(host); ip != nil {
			if ip4 := ip.To4(); ip4 != nil {
				ipAddrs = appendIfMissingIP(ipAddrs, ip4)
			} else {
				ipAddrs = appendIfMissingIP(ipAddrs, ip)
			}
		} else {
			dnsNames = appendIfMissingString(dnsNames, host)
		}
	}

	ipAddrs = appendIfMissingIP(ipAddrs, net.ParseIP("::1"))

	return dnsNames, ipAddrs
}

func appendIfMissingIP(list []net.IP, ip net.IP) []net.IP {
	if ip == nil {
		return list
	}
	for _, existing := range list {
		if existing.Equal(ip) {
			return list
		}
	}
	return append(list, ip)
}

func appendIfMissingString(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}

func parseDER(certPEM []byte) []byte {
	block, _ := pem.Decode(certPEM)
	if block != nil {
		return block.Bytes
	}
	return nil
}
