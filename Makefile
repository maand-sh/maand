IMAGE="maand"

build:
	docker build -t $(IMAGE) ./src/main

run_command:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) run_command $(ROLE)

bootstrap:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) bootstrap

linux_patching:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) linux_patching