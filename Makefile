.PHONY: test
test:
	go test -v -race .

.PHONY: bench
bench:
	go test -run ^$$ -bench . -benchmem -cpu=4 # -count=6

.PHONY: godoc-cli
godoc-cli:
	go build -o ./dist/godoc-cli ./cmd/godoc-cli

.PHONY: godoc-mcp
godoc-mcp:
	go build -o ./dist/godoc-mcp ./cmd/godoc-mcp

.PHONY: godoc-all
godoc-all: godoc-cli godoc-mcp