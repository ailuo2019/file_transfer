package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"fmt"	
	"flag"	
	"net/http"
	"os"
	"time"
	"strings"

	"github.com/pkg/errors"
	"context"
	"golang.org/x/net/http2"
)

// ClientH2 provides the implementation of a file
// uploader that streams data via an HTTP2-enabled
// connection.
type ClientH2 struct {
	client  *http.Client
	address string
}

type ClientH2Config struct {
	RootCertificate string
	Address         string
}

type Stats struct {
	StartedAt  time.Time
	FinishedAt time.Time
}

func must(err error) {
	if err == nil {
		return
	}

	fmt.Printf("ERROR: %+v\n", err)
	os.Exit(1)
}

func NewClientH2(cfg ClientH2Config) (c ClientH2, err error) {
	if cfg.Address == "" {
		err = errors.Errorf("Address must be non-empty")
		return
	}

	if cfg.RootCertificate == "" {
		err = errors.Errorf("RootCertificate must be specified")
		return
	}

	cert, err := ioutil.ReadFile(cfg.RootCertificate)
	if err != nil {
		err = errors.Wrapf(err,
			"failed to read root certificate")
		return
	}

	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(cert)
	if !ok {
		err = errors.Errorf(
			"failed to root certificate %s to cert pool",
			cfg.RootCertificate)
		return
	}

	c.client = &http.Client{
		Transport: &http2.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
	}

	c.address = cfg.Address

	return
}

func (c *ClientH2) UploadFile(ctx context.Context, f string) (stats Stats, err error) {
	var (
		file *os.File
	)

	file, err = os.Open(f)
	if err != nil {
		err = errors.Wrapf(err,
			"failed to open file %s", f)
		return
	}
	defer file.Close()
	subStr := strings.Split(f, "/")
	fileName := subStr[len(subStr)-1]
	req, err := http.NewRequest("POST", c.address+"/upload/" + fileName, file)
	if err != nil {
		err = errors.Wrapf(err,
			"failed to create POST request")
		return
	}

	stats.StartedAt = time.Now()
	resp, err := c.client.Do(req)
	if err != nil {
		err = errors.Wrapf(err,
			"request failed")
		return
	}
	stats.FinishedAt = time.Now()

	if resp.StatusCode != 200 {
		err = errors.Errorf("request failed - status code: %d",
			resp.StatusCode)
		return
	}

	return
}

func (c *ClientH2) Close() {
	return
}

func main() {
	addressPtr := flag.String("address", "localhost:1313", "address of the server to connect to")
	certPtr := flag.String("certificate", "./certs/selfsigned.cert", "path to TLS certificate")
	filePtr := flag.String("file", "", "file to upload")

	flag.Parse()
	cfg := ClientH2Config{}
	cfg.Address = *addressPtr
	cfg.RootCertificate = *certPtr
	if !strings.HasPrefix(cfg.Address, "https://") {
		cfg.Address = "https://" + cfg.Address
	}	

	file := *filePtr
	if file == "" {
		must(errors.New("file must be set"))
	}

	http2Client, err := NewClientH2(cfg)
	must(err)

	stat, err := http2Client.UploadFile(context.Background(), file)
	must(err)
	defer http2Client.Close()
	fmt.Printf("http2 upload file time %d ns\n", stat.FinishedAt.Sub(stat.StartedAt).Nanoseconds())

	return
}
