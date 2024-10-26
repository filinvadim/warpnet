package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	mathRand "math/rand/v2"
	"net"
	"time"
)

var rootCAs = x509.NewCertPool()

func GenerateTLSConfig(key, cert []byte) (*tls.Config, error) {
	// Загрузка сертификата и ключа
	certPem, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	// Создаем пул корневых сертификатов и добавляем наш самоподписанный сертификат
	if ok := rootCAs.AppendCertsFromPEM(cert); !ok {
		return nil, fmt.Errorf("failed to append cert to rootCAs")
	}
	// Создаем TLS-конфигурацию
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{certPem},
		RootCAs:            rootCAs,
		MinVersion:         tls.VersionTLS12,
		ClientAuth:         tls.RequireAndVerifyClientCert,
		InsecureSkipVerify: true,
	}

	return tlsConfig, nil
}

func GenerateSelfSignedCert(orgs ...string) (key []byte, cert []byte, err error) {
	// Генерация приватного ключа
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	// Параметры сертификата
	template := x509.Certificate{
		SerialNumber: big.NewInt(mathRand.Int64()),
		Subject: pkix.Name{
			Organization: orgs,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Срок действия 1 год
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")}, // Добавление IP в SAN
	}

	// Генерация самоподписанного сертификата
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %v", err)
	}

	bufCert := new(bytes.Buffer)
	if err := pem.Encode(bufCert, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return nil, nil, fmt.Errorf("failed to write data to cert.pem: %v", err)
	}

	privBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key: %v", err)
	}

	bufKey := new(bytes.Buffer)
	if err := pem.Encode(bufKey, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return nil, nil, fmt.Errorf("failed to write data to key.pem: %v", err)
	}

	return bufKey.Bytes(), bufCert.Bytes(), nil
}
