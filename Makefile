IMAGE="maand"

build:
	docker build -t $(IMAGE) ./src/main

run_command:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) run_command $(ROLE)

sync:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) sync

deploy-jobs:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) deploy_jobs

force-deploy-jobs:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) force_deploy_jobs

stop-jobs:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) stop_jobs

restart-jobs:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) restart_jobs

rolling-upgrade:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) rolling_upgrade

health-check:
	docker run --rm --privileged -e WORKSPACE=$(PWD)/workspace -v $(PWD)/workspace:/workspace -v /var/run/docker.sock:/var/run/docker.sock $(IMAGE) health_check
