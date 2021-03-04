package main

import (
	"io"
	"os"
	"time"
	"fmt"
	"flag"
	"strings"

	"example.com/file_transfer/messaging"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	_ "google.golang.org/grpc/encoding/gzip"
)

// ClientGRPC provides the implementation of a file
// uploader that streams chunks via protobuf-encoded
// messages.
type ClientGRPC struct {
	logger    zerolog.Logger
	conn      *grpc.ClientConn
	client    messaging.GuploadServiceClient
	chunkSize int
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

type ClientGRPCConfig struct {
	Address         string
	ChunkSize       int
	RootCertificate string
	Compress        bool
}

func NewClientGRPC(cfg ClientGRPCConfig) (c ClientGRPC, err error) {
	var (
		grpcOpts  = []grpc.DialOption{}
		grpcCreds credentials.TransportCredentials
	)

	if cfg.Address == "" {
		err = errors.Errorf("address must be specified")
		return
	}

	if cfg.Compress {
		grpcOpts = append(grpcOpts,
			grpc.WithDefaultCallOptions(grpc.UseCompressor("gzip")))
	}

	if cfg.RootCertificate != "" {
		grpcCreds, err = credentials.NewClientTLSFromFile(cfg.RootCertificate, "localhost")
		if err != nil {
			err = errors.Wrapf(err,
				"failed to create grpc tls client via root-cert %s",
				cfg.RootCertificate)
			return
		}

		grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(grpcCreds))
	} else {
		grpcOpts = append(grpcOpts, grpc.WithInsecure())
	}

	switch {
	case cfg.ChunkSize == 0:
		err = errors.Errorf("ChunkSize must be specified")
		return
	case cfg.ChunkSize > (1 << 22):
		err = errors.Errorf("ChunkSize must be < than 4MB")
		return
	default:
		c.chunkSize = cfg.ChunkSize
	}

	c.logger = zerolog.New(os.Stdout).
		With().
		Str("from", "client").
		Logger()

	c.conn, err = grpc.Dial(cfg.Address, grpcOpts...)
	if err != nil {
		err = errors.Wrapf(err,
			"failed to start grpc connection with address %s",
			cfg.Address)
		return
	}

	c.client = messaging.NewGuploadServiceClient(c.conn)

	return
}

func (c *ClientGRPC) UploadFile(ctx context.Context, f string) (stats Stats, err error) {
	var (
		writing = true
		buf     []byte
		n       int
		file    *os.File
		status  *messaging.UploadStatus
	)

	file, err = os.Open(f)
	if err != nil {
		err = errors.Wrapf(err,
			"failed to open file %s",
			f)
		return
	}
	defer file.Close()
	subStr := strings.Split(f, "/")
	fileName := subStr[len(subStr)-1]

	stream, err := c.client.Upload(ctx)
	if err != nil {
		err = errors.Wrapf(err,
			"failed to create upload stream for file %s",
			f)
		return
	}
	defer stream.CloseSend()

	stats.StartedAt = time.Now()
	buf = make([]byte, c.chunkSize)
	for writing {
		n, err = file.Read(buf)
		if err != nil {
			if err == io.EOF {
				writing = false
				err = nil
				continue
			}

			err = errors.Wrapf(err,
				"errored while copying from file to buf")
			return
		}

		err = stream.Send(&messaging.Chunk{
			Content: buf[:n],
			FileName: fileName,
		})

		if err != nil {
			err = errors.Wrapf(err,
				"failed to send chunk via stream")
			return
		}
	}

	stats.FinishedAt = time.Now()

	status, err = stream.CloseAndRecv()
	if err != nil {
		err = errors.Wrapf(err,
			"failed to receive upstream status response")
		return
	}

	if status.Code != messaging.UploadStatusCode_Ok {
		err = errors.Errorf(
			"upload failed - msg: %s",
			status.Message)
		return
	}

	return
}

func (c *ClientGRPC) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func main() {

	chunkSizePtr := flag.Int("chunk-size", (1<<12), "size of the chunk messages")
	addressPtr := flag.String("address", "localhost:1313", "path to TLS certificate")
	filePtr := flag.String("file", "", "file to upload")
	//example of default certificate: ./certs/selfsigned.cert
	//certPtr := flag.String("certificate", "", "path of a certificate to add to the root CAs")
	//compressPtr := flag.Bool("compress", false, "whether or not to enable payload compression")

	flag.Parse()
	cfg := ClientGRPCConfig{}

	cfg.Address = *addressPtr
	cfg.ChunkSize = *chunkSizePtr
	file := *filePtr
	if file == "" {
		must(errors.New("file must be set"))
	}
	//cfg.RootCertificate = *certPtr
	//cfg.Compress = *compressPtr

	grpcClient, err := NewClientGRPC(cfg)
	must(err)		
	stat, err := grpcClient.UploadFile(context.Background(), file)
	must(err)
	defer grpcClient.Close()
	fmt.Printf("gRPC upload file time %d ns\n", stat.FinishedAt.Sub(stat.StartedAt).Nanoseconds())
	return
}