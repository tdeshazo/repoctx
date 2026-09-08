.PHONY: test build example clean

test:
	go test ./...
	go vet ./...

build:
	go build -trimpath -o bin/repoctx .

example: build
	./bin/repoctx compile -root examples/mixed -o examples/mixed.ir.json.gz
	./bin/repoctx validate examples/mixed.ir.json.gz
	./bin/repoctx context -root examples/mixed -query 'Worker.files' -max-bytes 20000 -o examples/mixed.context.json examples/mixed.ir.json.gz

clean:
	rm -rf bin examples/mixed.ir.json.gz examples/mixed.context.json
