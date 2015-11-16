COW_PATH = say/internal/cows/
COW_FILES = $(wildcard $(COW_PATH)*.cow)
COW_BUILD = $(COW_PATH)cows.go

.PHONY: default test resetdb cows clean

default: test

test: cows
	go test ./...
	go vet ./...

resetdb:
	psql postgres -c 'DROP DATABASE IF EXISTS saypi'
	psql postgres -c 'CREATE DATABASE saypi'
	cat schema.sql | psql saypi

cows: $(COW_BUILD)

clean:
	rm $(COW_BUILD)

$(COW_BUILD): $(COW_FILES)
	go-bindata -o="$@" -ignore="$@" -pkg="cows" -nomemcopy -nometadata -prefix="$(COW_PATH)" "$(COW_PATH)"
