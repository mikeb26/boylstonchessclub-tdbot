export GO111MODULE=on
export GOFLAGS=-mod=vendor

.PHONY: build
build: discordbot

vendor: go.mod
	go mod download
	go mod vendor

discordbot: vendor FORCE
	go build github.com/mikeb26/boylstonchessclub-tdbot/cmd/discordbot

test: build FORCE
	go test github.com/mikeb26/boylstonchessclub-tdbot/cmd/discordbot

.PHONY: deps
deps:
	rm -rf go.mod go.sum vendor
	go mod init github.com/mikeb26/boylstonchessclub-tdbot
	GOPROXY=direct go mod tidy
	go mod vendor

.PHONY: clean
clean:
	rm -rf discordbot

FORCE:
