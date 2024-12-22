build:
	docker build -t maand .

exec:
	docker run --rm --user=root --entrypoint=/bin/bash -v $(PWD)/bucket:/bucket:z -it maand

alias:
	alias maand="docker run --rm -v $(PWD)/bucket:/bucket:z maand "
