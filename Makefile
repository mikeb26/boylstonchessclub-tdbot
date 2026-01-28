export GO111MODULE=on
export GOFLAGS=-mod=vendor

.PHONY: build
build: discordbot bcctd cacheseed

vendor: go.mod
	go mod download
	go mod vendor

discordbot: vendor FORCE
	go build github.com/mikeb26/boylstonchessclub-tdbot/cmd/discordbot

bcctd: vendor FORCE
	go build github.com/mikeb26/boylstonchessclub-tdbot/cmd/bcctd

cacheseed: vendor FORCE
	go build github.com/mikeb26/boylstonchessclub-tdbot/cmd/cacheseed

test: build FORCE
	go test github.com/mikeb26/boylstonchessclub-tdbot/cmd/discordbot
	go test github.com/mikeb26/boylstonchessclub-tdbot/bcc
	go test github.com/mikeb26/boylstonchessclub-tdbot/uschess
	go test github.com/mikeb26/boylstonchessclub-tdbot/internal
	go test github.com/mikeb26/boylstonchessclub-tdbot/internal/httpcache
	go test github.com/mikeb26/boylstonchessclub-tdbot/s3cache

.PHONY: deps
deps:
	rm -rf go.mod go.sum vendor
	go mod init github.com/mikeb26/boylstonchessclub-tdbot
	go mod edit -replace=github.com/bwmarrin/discordgo=github.com/mikeb26/bwmarrin-discordgo@v0.29.0.mb1
	GOPROXY=direct go mod tidy
	go mod vendor

.PHONY: clean
clean:
	rm -rf discordbot bcctd cacheseed

FORCE:
