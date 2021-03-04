module example.com/file_transfer/grpc_client

go 1.13

require (
	example.com/file_transfer v0.0.0-00010101000000-000000000000
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.20.0
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	google.golang.org/grpc v1.36.0
)

replace example.com/file_transfer => ../
