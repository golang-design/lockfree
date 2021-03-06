# Copyright 2020 The golang.design Initiative authors.
# All rights reserved. Use of this source code is governed
# by a MIT license that can be found in the LICENSE file.

COUNT := $(if $(COUNT),$(COUNT),1)

all:
	@echo "Usage:"
	@echo "  - make tests: run all tests with race detection"
	@echo "  - make bench count=<number>: run all benches with given count"

tests:
	go test -race -cover -coverprofile=cover.test -v .
	go tool cover -html=cover.test -o cover.html

bench:
	go test -bench=. -benchmem -count=$(COUNT)