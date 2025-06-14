BINS = linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64

all: $(BINS)

linux-amd64:
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -tags avx2 -o build/torwell84-linux-amd64

linux-arm64:
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -tags neon -o build/torwell84-linux-arm64

darwin-amd64:
GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -tags avx2 -o build/torwell84-darwin-amd64

darwin-arm64:
GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -tags neon -o build/torwell84-darwin-arm64

windows-amd64:
GOOS=windows GOARCH=amd64 go build -o build/torwell84-windows-amd64.exe

clean:
rm -rf build
