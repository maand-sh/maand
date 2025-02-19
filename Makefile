build:
	go build -o maand_base

docker:
	docker build -t maand_base .