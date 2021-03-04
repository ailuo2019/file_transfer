package main

import (
	"flag"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

func main() {
	commonNamePtr := flag.String("common_name", "tencent", "Common name. Can be anything")
	ipPtr := flag.String("ip", "", "ip address of the server")
	certKeyNamePtr := flag.String("cert-key-name","selfsigned","The certifcate and key will use the same name: xxx.cert, xxx.key")
	validDaysPtr := flag.Int("valid-days", 3650, "the number of days the certification and key are valid")
	flag.Parse()

	var err error

	template := x509.Certificate{
		Subject: pkix.Name{
			Organization: []string{"Log Courier"},
		},
		NotBefore: time.Now(),

		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,

		IsCA: true,
	}

	template.Subject.CommonName = *commonNamePtr
	ipStr := *ipPtr
	if ip := net.ParseIP(ipStr); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		fmt.Printf("the IP address %s is invalid!\n", ipStr)
		os.Exit(1)
	}

	template.NotAfter = template.NotBefore.Add(time.Duration(*validDaysPtr) * time.Hour * 24)

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println("Failed to generate private key:", err)
		os.Exit(1)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	template.SerialNumber, err = rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		fmt.Println("Failed to generate serial number:", err)
		os.Exit(1)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		fmt.Println("Failed to create certificate:", err)
		os.Exit(1)
	}
	certKeyName := *certKeyNamePtr
	certOut, err := os.Create(certKeyName + ".cert")
	if err != nil {
		fmt.Printf("Failed to open %s.cert for writing:\n", certKeyName)
		os.Exit(1)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, err := os.OpenFile(certKeyName + ".key", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Printf("failed to open %s.key for writing:\n", certKeyName)
		os.Exit(1)
	}
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	keyOut.Close()

	fmt.Println("Successfully generated certificate")
	fmt.Printf("    Certificate: %s.cert\n", certKeyName)
	fmt.Printf("    Private Key: %s.key\n",certKeyName)
	fmt.Println()
	fmt.Println("Copy and paste the following into your Log Courier")
	fmt.Println("configuration, adjusting paths as necessary:")
	fmt.Println("    \"transport\": \"tls\",")
	fmt.Printf("    \"ssl ca\":    \"path/to/%s.cert\",\n", certKeyName)
	fmt.Println()
	fmt.Println("Copy and paste the following into your LogStash configuration, ")
	fmt.Println("adjusting paths as necessary:")
	fmt.Printf("    ssl_certificate => \"path/to/%s.cert\",\n", certKeyName)
	fmt.Printf("    ssl_key         => \"path/to/%s.key\",\n", certKeyName)
}
