build:
	go build
test:
	docker system prune -af
	go test -v ./tests
