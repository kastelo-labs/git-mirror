all: git-mirror-linux-amd64

git-mirror-linux-amd64:
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -o git-mirror-linux-amd64

