package main

import (
	"io"
//	"io/ioutil"	
	"net"
	"os"
	"strconv"
	"fmt"
	"flag"
//	"strings"

	"example.com/file_transfer/messaging"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	_ "google.golang.org/grpc/encoding/gzip"
)

type ServerGRPC struct {
	logger      zerolog.Logger
	server      *grpc.Server
	port        int
	certificate string
	key         string
	SaveDir     string
}

func must(err error) {
	if err == nil {
		return
	}

	fmt.Printf("ERROR: %+v\n", err)
	os.Exit(1)
}

type ServerGRPCConfig struct {
	Certificate string
	Key         string
	SaveDir     string
	Port        int
}

func NewServerGRPC(cfg ServerGRPCConfig) (s ServerGRPC, err error) {
	s.logger = zerolog.New(os.Stdout).
		With().
		Str("from", "server").
		Logger()

	if cfg.Port == 0 {
		err = errors.Errorf("Port must be specified")
		return
	}

	s.port = cfg.Port
	s.certificate = cfg.Certificate
	s.key = cfg.Key
	s.SaveDir = cfg.SaveDir

	return
}

func (s *ServerGRPC) Listen() (err error) {
	var (
		listener  net.Listener
		grpcOpts  = []grpc.ServerOption{}
		grpcCreds credentials.TransportCredentials
	)

	listener, err = net.Listen("tcp", ":"+strconv.Itoa(s.port))
	if err != nil {
		err = errors.Wrapf(err,
			"failed to listen on port %d",
			s.port)
		return
	}

	if s.certificate != "" && s.key != "" {
		grpcCreds, err = credentials.NewServerTLSFromFile(
			s.certificate, s.key)
		if err != nil {
			err = errors.Wrapf(err,
				"failed to create tls grpc server using cert %s and key %s",
				s.certificate, s.key)
			return
		}

		grpcOpts = append(grpcOpts, grpc.Creds(grpcCreds))
	}

	s.server = grpc.NewServer(grpcOpts...)
	messaging.RegisterGuploadServiceServer(s.server, s)

	err = s.server.Serve(listener)
	if err != nil {
		err = errors.Wrapf(err, "errored listening for grpc connections")
		return
	}

	return
}

func (s *ServerGRPC) Upload(stream messaging.GuploadService_UploadServer) (err error) {
	req, err := stream.Recv()
	fileName := req.GetFileName()
	outputFile, err1 := os.Create(s.SaveDir+fileName)
	must(err1)
	defer outputFile.Close()
	for {
		if err != nil {
			if err == io.EOF {
				goto END
			}

			err = errors.Wrapf(err,
				"failed unexpectadely while reading chunks from stream")
			return
		}
		chunk := req.GetContent()
		_, err1 := outputFile.Write(chunk)
		must(err1)
		req, err = stream.Recv()
	}

END:
	outputFile.Sync()
	fmt.Printf("file %s is uploaded\n", s.SaveDir+fileName)
	err = stream.SendAndClose(&messaging.UploadStatus{
		Message: "Upload received with success",
		Code:    messaging.UploadStatusCode_Ok,
	})
	if err != nil {
		err = errors.Wrapf(err,
			"failed to send status code")
		return
	}

	return
}

func (s *ServerGRPC) Close() {
	if s.server != nil {
		s.server.Stop()
	}

	return
}

func main() {

	portPtr := flag.Int("port", 1313, "port to bind to")
	//example of default key: ./certs/selfsigned.key	
	//keyPtr := flag.String("key", "", "path to TLS certificate")
	//example of default certificate: ./certs/selfsigned.cert
	//certPtr := flag.String("certificate", "", "path to TLS certificate")
	saveDirPtr := flag.String("dir", "./", "path to save the uploaded files")
	flag.Parse()

	ServerCfg := ServerGRPCConfig{}
	ServerCfg.Port = *portPtr;
	ServerCfg.SaveDir = *saveDirPtr
	//ServerCfg.Certificate = *certPtr
	//ServerCfg.Key = *keyPtr

	grpcServer, err := NewServerGRPC(ServerCfg)
	must(err)		
	err = grpcServer.Listen()
	must(err)
	defer grpcServer.Close()
	return
}