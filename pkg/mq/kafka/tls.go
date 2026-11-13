package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func decryptPEM(data, passphrase []byte) ([]byte, error) {
	if len(passphrase) == 0 {
		return data, nil
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("kafka tls: pem decode failed")
	}
	decrypted, err := x509.DecryptPEMBlock(block, passphrase)
	if err != nil {
		return nil, fmt.Errorf("kafka tls: decrypt pem: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: block.Type, Bytes: decrypted}), nil
}

func readEncryptablePEMBlock(path string, pwd []byte) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("kafka tls: read %s: %w", path, err)
	}
	return decryptPEM(data, pwd)
}

func newTLSConfig(clientCertFile, clientKeyFile, caCertFile string, keyPwd []byte, insecureSkipVerify bool) (*tls.Config, error) {
	var cfg tls.Config
	if clientCertFile != "" && clientKeyFile != "" {
		certPEM, err := os.ReadFile(clientCertFile)
		if err != nil {
			return nil, fmt.Errorf("kafka tls: read client cert: %w", err)
		}
		keyPEM, err := readEncryptablePEMBlock(clientKeyFile, keyPwd)
		if err != nil {
			return nil, err
		}
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("kafka tls: x509 key pair: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	if caCertFile != "" {
		caCert, err := os.ReadFile(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("kafka tls: read ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("kafka tls: append ca certs failed")
		}
		cfg.RootCAs = pool
	}
	cfg.InsecureSkipVerify = insecureSkipVerify
	return &cfg, nil
}
