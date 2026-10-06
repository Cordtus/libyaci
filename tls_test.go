package libyaci

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
)

func TestWithTLSConfig_Option(t *testing.T) {
	o := defaultOptions()
	cfg := &tls.Config{InsecureSkipVerify: true}
	WithTLSConfig(cfg)(o)
	if o.tlsConfig != cfg {
		t.Fatal("WithTLSConfig should set the TLS config")
	}
}

func TestDialWithCustomTLS(t *testing.T) {
	cert := generateSelfSignedCert(t)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}})))
	reflectionpb.RegisterServerReflectionServer(server, &mockReflectionServer{})
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	ctx := context.Background()

	// Default credentials must reject the self-signed certificate.
	if _, err := Dial(ctx, lis.Addr().String(), WithDialTimeout(3*time.Second)); err == nil {
		t.Fatal("dial should fail without a trusted certificate")
	}

	// A custom TLS config that skips verification must succeed.
	client, err := Dial(ctx, lis.Addr().String(),
		WithDialTimeout(5*time.Second),
		WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
	)
	if err != nil {
		t.Fatalf("dial with custom TLS failed: %v", err)
	}
	defer client.Close()

	if !client.SupportsMethod("test.TestService.TestMethod") {
		t.Fatal("reflection should work over the custom TLS connection")
	}
}

func generateSelfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	return cert
}
