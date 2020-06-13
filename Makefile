fmt:
	go fmt ./...
	go vet ./...

test:
	go test -v -race -count=100

bench:
	go test ./... -bench=. -benchtime=10s
