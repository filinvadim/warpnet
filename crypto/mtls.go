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
	"time"
)

func GenerateTLSConfig(orgs ...string) (*tls.Config, error) {
	key, cert, err := generateSelfSignedCert(orgs...)
	if err != nil {
		return nil, fmt.Errorf("failed generating certificate: %w", err)
	}

	// Загрузка сертификата и ключа
	certPem, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	// Создаем пул корневых сертификатов и добавляем наш самоподписанный сертификат
	rootCAs := x509.NewCertPool()
	if ok := rootCAs.AppendCertsFromPEM(cert); !ok {
		return nil, fmt.Errorf("failed to append cert to rootCAs")
	}

	// Создаем TLS-конфигурацию
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certPem},
		RootCAs:      rootCAs, // Доверяем этому сертификату как корневому
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	return tlsConfig, nil
}

func generateCA(commonName string, orgs ...string) ([]byte, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(mathRand.Int64()), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %v", err)
	}

	// Параметры CA-сертификата
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: orgs,
			Country:      []string{"Worldwide"},
			CommonName:   commonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // Действителен 10 лет
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true, // Устанавливаем как CA
		BasicConstraintsValid: true,
		MaxPathLen:            1, // Может подписывать подчиненные сертификаты
	}

	// Создание самоподписанного сертификата для CA
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA certificate: %v", err)
	}

	// Кодируем сертификат в PEM-формат
	bufCert := new(bytes.Buffer)
	err = pem.Encode(bufCert, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to write CA certificate: %v", err)
	}

	// Кодируем приватный ключ в PEM-формат
	privBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key: %v", err)
	}

	bufKey := new(bytes.Buffer)
	err = pem.Encode(bufKey, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to write private key: %v", err)
	}

	return bufKey.Bytes(), bufCert.Bytes(), nil
}

func generateSelfSignedCert(orgs ...string) (key []byte, cert []byte, err error) {
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
