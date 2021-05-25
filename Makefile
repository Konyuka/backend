APP = smartdial

fmt:
	go fmt ./...

run:
	go run main.go s

build: fmt
	go mod tidy
	go build -o ${APP} .

cron:
	go run main.go cron

test:
	go test tests/* -v

deploy: build
	scp smartdial root@172.16.10.209:test letmepass


