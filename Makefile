.PHONY: run bench

run:
	go run .

bench:
	go test -bench=. -benchmem -run=^$$ ./...
