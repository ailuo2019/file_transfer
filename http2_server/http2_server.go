package main

import (
	"bytes"
	"fmt"
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/net/http2"
)

type ServerH2 struct {
	server      *http.Server
	logger      zerolog.Logger
	certificate string
	key         string
	SaveDir     string
}

type ServerH2Config struct {
	Port        int
	Certificate string
	Key         string
	SaveDir     string

}

func must(err error) {
	if err == nil {
		return
	}

	fmt.Printf("ERROR: %+v\n", err)
	os.Exit(1)
}

func NewServerH2(cfg ServerH2Config) (s ServerH2, err error) {
	if cfg.Port == 0 {
		err = errors.Errorf("Port must be non-zero")
		return
	}

	if cfg.Certificate == "" {
		err = errors.Errorf("Certificate must be specified")
		return
	}

	if cfg.Key == "" {
		err = errors.Errorf("Key must be specified")
		return
	}

	s.logger = zerolog.New(os.Stdout).
		With().
		Str("from", "server_h2").
		Logger()

	s.server = &http.Server{
		Addr: ":" + strconv.Itoa(cfg.Port),
	}

	s.certificate = cfg.Certificate
	s.key = cfg.Key
	s.SaveDir = cfg.SaveDir

	http2.ConfigureServer(s.server, nil)
	http.HandleFunc("/upload/", s.Upload)

	return
}

func (s *ServerH2) Listen() (err error) {
	err = s.server.ListenAndServeTLS(
		s.certificate, s.key)
	if err != nil {
		err = errors.Wrapf(err, "failed during server listen and serve")
		return
	}

	return
}

func (s *ServerH2) Upload(w http.ResponseWriter, r *http.Request) {
	var (
		err           error
		bytesReceived int64 = 0
		buf                 = new(bytes.Buffer)
	)

	bytesReceived, err = io.Copy(buf, r.Body)
	subStr := strings.Split(r.URL.Path, "/")
	fileName := subStr[len(subStr)-1]
	err = ioutil.WriteFile(s.SaveDir + fileName,buf.Bytes(), 0666)
	if err != nil {
		s.logger.Error().
			Err(err).
			Msg("failed to copy body into buf")

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "%+v", err)
		return
	}

	// just receives the content and prints to stdout
	// read the body.


	fmt.Printf("file %s is uploaded\n", s.SaveDir+fileName)
	s.logger.Info().
		Int64("bytes_received", bytesReceived).
		Msg("upload received")

	return
}

func (s *ServerH2) Close() {
	return
}

func main() {

	portPtr := flag.Int("port", 1313, "port to bind to")
	keyPtr := flag.String("key", "./certs/selfsigned.key", "path to TLS certificate")
	certPtr := flag.String("certificate", "./certs/selfsigned.cert", "path to TLS certificate")
	saveDirPtr := flag.String("dir", "./", "path to save the uploaded files")

	flag.Parse()

	ServerCfg := ServerH2Config{}
	ServerCfg.Port = *portPtr;
	ServerCfg.Certificate = *certPtr
	ServerCfg.Key = *keyPtr
	ServerCfg.SaveDir = *saveDirPtr
	
	http2Server, err := NewServerH2(ServerCfg)
	must(err)		
	err = http2Server.Listen()
	must(err)
	defer http2Server.Close()
	return
}