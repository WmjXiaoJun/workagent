# WorkAgent —— 常用任务
BIN     := workagent
CMD     := ./cmd/workagent
DIST    := dist

.PHONY: all build test vet fmt check run verify seal adapt attest release clean

all: vet test

build:
	go build -trimpath -o $(BIN) $(CMD)

vet:
	go vet ./...

fmt:
	gofmt -w ./cmd

test: build
	CGO_ENABLED=0 go test ./... -count=1

check: build
	./$(BIN) check .

attest: build
	./$(BIN) attest .

adapt: build
	./$(BIN) adapt .

run: build
	./$(BIN) run --input examples/bid_quote/input.json .

verify: build
	./$(BIN) verify .

seal: build
	./$(BIN) seal .

# 一条命令产出五平台静态二进制
release:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-linux-amd64     $(CMD)
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-linux-arm64     $(CMD)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-darwin-arm64    $(CMD)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-darwin-amd64    $(CMD)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-windows-amd64.exe $(CMD)
	@ls -la $(DIST)

clean:
	rm -rf $(BIN) $(BIN).exe $(DIST) out
