BIN := openaula
PREFIX ?= /usr/local

.PHONY: build test vet fmt install udev clean

build:
	go build -trimpath -ldflags "-s -w" -o $(BIN) ./cmd/openaula

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

install: build
	install -Dm755 $(BIN) $(DESTDIR)$(PREFIX)/bin/$(BIN)

udev:
	install -Dm644 packaging/udev/99-openaula.rules /etc/udev/rules.d/99-openaula.rules
	udevadm control --reload-rules
	udevadm trigger

clean:
	rm -f $(BIN)
